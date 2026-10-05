# SPDX-License-Identifier: GPL-3.0-or-later
use strict;
use warnings;
use JSON::PP;
use Digest::SHA qw(sha256_hex);
use POSIX qw(strftime);
use File::Temp qw(tempfile);
use File::Basename qw(dirname);
use Fcntl qw(O_RDONLY O_NOFOLLOW);

require '../web-lib.pl' if -f '../web-lib.pl';
our (%config, %in, %text, %access, $module_config_directory, $module_name);

sub sth_init {
 &init_config();
 &ReadParse();
 %access = &get_module_acl();
}
sub sth_require {
 my ($right) = @_;
 &error('Permission denied') unless $access{$right};
}
sub sth_require_post { &error('POST request required') unless ($ENV{'REQUEST_METHOD'}||'') eq 'POST'; }
sub sth_escape { my ($s)=@_; $s='' unless defined $s; return &html_escape($s); }
sub sth_sanitize_log {
 my ($s)=@_; $s='' unless defined $s;
 $s =~ s/\x00|[\x01-\x08\x0b\x0c\x0e-\x1f\x7f]/ /g;
 $s =~ s/api-[A-Za-z0-9]{32}/[redacted]/g;
 $s =~ s/(?:authorization|x-smtp2go-api-key|api[_-]?key|token|secret)\s*[:= ]\s*[^\s,;]+/[credential redacted]/ig;
 $s =~ s/[A-Za-z0-9+\/] {80,}={0,2}/[data redacted]/gx;
 $s =~ s/"(?:mime_email|payload|fileblob|attachments?|body|html_body|text_body)"\s*:\s*(?:"(?:\\.|[^"\\])*"|[^,}\s]+)/"sensitive" : "[redacted]"/ig;
 $s =~ s/[\r\n\t]+/ /g;
 $s=substr($s,0,500);
 return $s;
}
sub sth_t { my ($key,$fallback)=@_; return defined($text{$key}) ? $text{$key} : $fallback; }
sub sth_capture {
 my (@cmd)=@_;
 my $pid=open(my $fh, '-|', @cmd);
 return (127, '') unless $pid;
 local $/; my $data=<$fh>; close($fh);
 return ($? >> 8, defined($data)?$data:'');
}
sub sth_capture_limited {
 my ($limit,@cmd)=@_;
 my $pid=open(my $fh, '-|', @cmd);
 return (127,'') unless $pid;
 binmode($fh);
 my ($data,$total)=('',0);
 while (1) {
   my $n=sysread($fh,my $chunk,4096);
   last unless defined($n) && $n>0;
   $total += $n;
   if (length($data)<$limit) { $data .= substr($chunk,0,$limit-length($data)); }
 }
 close($fh);
 $data .= "\n[output truncated]" if $total>$limit;
 return ($? >> 8,$data);
}
sub sth_read_config {
 my $path=$config{'config_file'} || '/etc/smtp2go-helper/config.json';
 my $gid=(getgrnam('smtp2go-helper'))[2]; return unless defined $gid;
 my @ds=lstat(dirname($path)); return unless @ds && -d _ && !-l _ && $ds[4]==0 && $ds[5]==$gid && ($ds[2]&07777)==0750;
 my @ls=lstat($path); return unless @ls && -f _ && !-l _ && $ls[4]==0 && $ls[5]==$gid && ($ls[2]&07777)==0640;
 sysopen(my $fh, $path, O_RDONLY|O_NOFOLLOW) or return;
 my @st=stat($fh); return unless @st && -f $fh && $st[4]==0 && ($st[2]&07777)==0640 && $st[7]<=65536;
 local $/; my $raw=<$fh>; close($fh);
 my $obj=eval { JSON::PP->new->decode($raw) };
 return $@ ? undef : $obj;
}
sub sth_write_atomic {
 my ($path,$data,$mode,$group)=@_;
 my $dir=dirname($path);
 my @ds=lstat($dir); die 'unsafe destination directory' unless defined($group) && @ds && -d _ && !-l _ && $ds[4]==0 && $ds[5]==$group && ($ds[2]&07777)==0750;
 die 'unsafe destination file' if -l $path;
 my ($fh,$tmp)=tempfile('.smtp2go-helper-XXXXXXXX',DIR=>dirname($path),UNLINK=>0);
 my $ok=eval {
   binmode($fh) or die 'cannot set temporary config mode';
   chmod(0600,$tmp) or die 'cannot restrict temporary config';
   print {$fh} $data or die 'cannot write config';
   close($fh) or die 'cannot close config';
   chown(0,$group,$tmp)==1 or die 'cannot set config owner';
   chmod($mode,$tmp) or die 'cannot set config permissions';
   rename($tmp,$path) or die 'cannot replace config';
   1;
 };
 unless ($ok) { close($fh); unlink($tmp) if defined($tmp) && -e $tmp; die 'atomic protected write failed'; }
}
sub sth_read_key_secure {
 my $p=$config{'key_file'} || '/etc/smtp2go-helper/api.key';
 sysopen(my $fh,$p,O_RDONLY|O_NOFOLLOW) or return;
 my @s=stat($fh); return unless @s && -f $fh && ($s[2]&07777)==0640 && $s[4]==0 && $s[5]==(getgrnam('smtp2go-helper'))[2] && $s[7]<=256;
 local $/; my $key=<$fh>; close($fh); $key =~ s/[\r\n]+$//;
 return unless $key =~ /^api-[A-Za-z0-9]{32}$/ && $key !~ /\x00/;
 return $key;
}
sub sth_key_meta {
 my $p=$config{'key_file'} || '/etc/smtp2go-helper/api.key';
 return (0,'') unless -f $p && !-l $p;
 sysopen(my $fh,$p,O_RDONLY|O_NOFOLLOW) or return (0,'');
 my @s=stat($fh); return (0,'') unless @s && -f $fh && ($s[2]&07777)==0640 && $s[4]==0 && $s[5]==(getgrnam('smtp2go-helper'))[2] && $s[7]<=256;
 local $/; my $key=<$fh>; close($fh); $key =~ s/[\r\n]+$//;
 return (0,'') unless $key =~ /^api-[A-Za-z0-9]{32}$/ && $key !~ /\x00/;
 return (1,substr(sha256_hex($key),0,16));
}
sub sth_queue {
 my ($status,$out)=sth_capture_limited(262144,'/usr/sbin/postqueue','-p');
 return ('unknown','') if $status;
 my ($active,$hold,$deferred)=(0,0,0);
 for my $line (split /\n/,$out) {
   next unless $line =~ /^([A-F0-9]{10})([!*]?)\s/;
   my $marker=$2;
   $marker eq '!' ? $hold++ : $marker eq '*' ? $active++ : $deferred++;
 }
 return ($active,$hold,$deferred);
}
sub sth_postconf {
 my ($name)=@_; my ($rc,$out)=sth_capture('/usr/sbin/postconf','-h',$name);
 return $rc ? '' : $out =~ s/\s+$//r;
}
sub sth_status {
 my ($vrc,$version)=sth_capture($config{'helper_bin'}||'/usr/local/libexec/smtp2go-helper','--version');
 my $cfg=sth_read_config()||{};
 my ($configured,$fp)=sth_key_meta();
 my ($prc,$pstatus)=sth_capture('/bin/systemctl','is-active','postfix');
 my ($pr,$perm)=sth_capture($config{'helper_bin'}||'/usr/local/libexec/smtp2go-helper','api','permissions');
 my ($a,$h,$d)=sth_queue();
 my $sender=$cfg->{default_sender}//'';
 my $domain=$sender =~ /\@([^\s>]+)/ ? $1 : '';
 my ($lrc,$logs)=sth_capture_limited(64000,'/usr/bin/journalctl','-u','postfix','-n','100','--no-pager','-o','cat');
 my ($last)=grep { /smtp2go-helper:/ } split /\n/,$logs;
 $last='' unless defined $last;
 $last=sth_sanitize_log($last);
 my ($src,$sockets)=sth_capture('/usr/bin/ss','-lnt');
 my $listener='unknown';
 if (!$src) {
   $listener='loopback-only';
   for my $line (split /\n/,$sockets) { my @f=split /\s+/,$line; if (@f>3 && $f[0] eq 'LISTEN' && $f[3] =~ /:25$/ && $f[3] !~ /^(127\.0\.0\.1:\d+|\[::1\]:\d+)$/) { $listener='external'; } }
 }
 return {
  version=>$vrc?'unavailable':$version, postfix=>$prc?'unknown':$pstatus,
  endpoint=>$cfg->{endpoint}//'', key=>$configured?'configured':'not configured',
  key_fingerprint=>$configured?$fp:'', permission=>$pr?'unknown':($perm=~/allowed/?'allowed':sth_sanitize_log($perm)),
  recipient_limit=>sth_postconf('smtp2go-helper_destination_recipient_limit'),
  transport=>sth_postconf('default_transport'), sender=>$cfg->{default_sender}//'',
  sender_domain=>$domain, listener=>$listener, last_result=>$last,
  queue_active=>$a, queue_hold=>$h, queue_deferred=>$d,
 };
}
1;
