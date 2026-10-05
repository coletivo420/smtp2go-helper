#!/usr/bin/perl
use strict;
use warnings;
use IPC::Open3;
use Symbol qw(gensym);
require './smtp2go-helper-lib.pl';
our (%in,%config);
&sth_init();
&sth_require('view');
&sth_require('test') if $in{'send'};
my $message='';
my $sent=0;
if ($in{'send'}) {
	&sth_require_post();
 my ($from,$to)=($in{'from'}||'',$in{'to'}||'');
 my ($subject,$body)=($in{'subject'}||'SMTP2GO Helper test',$in{'body'}||'SMTP2GO Helper test message');
 if ($from !~ /^[^\s<>\r\n]+\@[^\s<>\r\n]+$/ || $to !~ /^[^\s<>\r\n]+\@[^\s<>\r\n]+$/ ||
     $subject =~ /[\r\n\x00]/ || $body =~ /\x00/ || length($body)>4096) {
   $message='Invalid sender, recipient, subject, or body';
 } else {
   my $err=gensym;
   my ($child_in,$child_out);
   my $pid=eval { open3($child_in,$child_out,$err,$config{'sendmail_bin'}||'/usr/sbin/sendmail','-i','-f',$from,'--',$to) };
   if (!$pid) { $message='Could not start local sendmail'; }
   else {
     my $mime="From: $from\r\nTo: $to\r\nSubject: $subject\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n$body\r\n";
     print {$child_in} $mime; close($child_in); close($child_out);
     local $/; my $errout=<$err>; close($err); waitpid($pid,0);
     if ($?==0) { $sent=1; $message='Submitted through local sendmail; check Postfix queue/logs for delivery result'; }
     else { $message='sendmail submission failed'; }
   }
 }
}
&ui_print_header(undef,'Send test via Postfix','');
print '<p>'.&sth_escape($message).'</p>' if $message;
print &ui_form_start('test.cgi','post');
print '<input type="hidden" name="send" value="1"><table>';
for my $field (['From','from'],['To','to'],['Subject','subject']) { print '<tr><th>'.$field->[0].'</th><td><input name="'.$field->[1].'" value="'.&sth_escape($in{$field->[1]}||'').'"></td></tr>'; }
print '<tr><th>Body</th><td><textarea name="body" rows="5" cols="60">'.&sth_escape($in{'body'}||'').'</textarea></td></tr></table><input type="submit" value="Send via Postfix"></form>';
&ui_print_footer('index.cgi');
