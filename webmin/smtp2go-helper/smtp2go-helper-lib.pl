# SPDX-License-Identifier: GPL-3.0-or-later
use strict;
use warnings;
use JSON::PP;
use Digest::SHA qw(sha256_hex);
use POSIX qw(strftime);

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
sub sth_escape { my ($s)=@_; $s='' unless defined $s; return &html_escape($s); }
sub sth_t { my ($key,$fallback)=@_; return defined($text{$key}) ? $text{$key} : $fallback; }
sub sth_capture {
 my (@cmd)=@_;
 my $pid=open(my $fh, '-|', @cmd);
 return (127, '') unless $pid;
 local $/; my $data=<$fh>; close($fh);
 return ($? >> 8, defined($data)?$data:'');
}
sub sth_read_config {
 my $path=$config{'config_file'} || '/etc/smtp2go-helper/config.json';
 open(my $fh, '<', $path) or return;
 local $/; my $raw=<$fh>; close($fh);
 my $obj=eval { JSON::PP->new->decode($raw) };
 return $@ ? undef : $obj;
}
sub sth_write_atomic {
 my ($path,$data,$mode,$group)=@_;
 my $tmp=$path.'.tmp.'.$$;
 sysopen(my $fh,$tmp,0x41,0600) or die 'cannot create temporary config';
 binmode($fh); print {$fh} $data or die 'cannot write config';
 close($fh) or die 'cannot close config';
 chmod($mode,$tmp) or die 'cannot set config permissions';
 chown(0,$group,$tmp) if defined $group;
 rename($tmp,$path) or die 'cannot replace config';
}
sub sth_key_meta {
 my $p=$config{'key_file'} || '/etc/smtp2go-helper/api.key';
 return (0,'') unless -f $p && !-l $p;
 my @s=stat($p); return (0,'') unless @s && ($s[2]&07777) == 0640 && $s[4]==0;
 open(my $fh,'<',$p) or return (0,'');
 local $/; my $key=<$fh>; close($fh); $key =~ s/[\r\n]+$//;
 return (0,'') unless $key =~ /^api-[A-Za-z0-9]{32}$/;
 return (1,substr(sha256_hex($key),0,16));
}
sub sth_queue {
 my ($status,$out)=sth_capture('/usr/sbin/postqueue','-p');
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
 my ($lrc,$logs)=sth_capture('/usr/bin/journalctl','-u','postfix','-n','100','--no-pager','-o','cat');
 my ($last)=grep { /smtp2go-helper:/ } split /\n/,$logs;
 $last='' unless defined $last;
 $last =~ s/api-[A-Za-z0-9]{32}/[redacted]/g;
 $last =~ s/[\r\n\t]/ /g;
 $last=substr($last,0,300);
 my ($src,$sockets)=sth_capture('/usr/bin/ss','-lnt');
 my $listener='unknown';
 if (!$src) {
   $listener='loopback-only';
   for my $line (split /\n/,$sockets) { my @f=split /\s+/,$line; if (@f>3 && $f[0] eq 'LISTEN' && $f[3] =~ /:25$/ && $f[3] !~ /^(127\.0\.0\.1:\d+|\[::1\]:\d+)$/) { $listener='external'; } }
 }
 return {
  version=>$vrc?'unavailable':$version, postfix=>$prc?'unknown':$pstatus,
  endpoint=>$cfg->{endpoint}//'', key=>$configured?'configured':'not configured',
  key_fingerprint=>$configured?$fp:'', permission=>$pr?'unknown':($perm=~/allowed/?'allowed':$perm),
  recipient_limit=>sth_postconf('smtp2go-helper_destination_recipient_limit'),
  transport=>sth_postconf('default_transport'), sender=>$cfg->{default_sender}//'',
  sender_domain=>$domain, listener=>$listener, last_result=>$last,
  queue_active=>$a, queue_hold=>$h, queue_deferred=>$d,
 };
}
1;
