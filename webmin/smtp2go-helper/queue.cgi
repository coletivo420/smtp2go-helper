#!/usr/bin/perl
use strict;
use warnings;
require './smtp2go-helper-lib.pl';
our (%in,%config,%access);
&sth_init();
&sth_require('queue_view');
&sth_require('queue_modify') if $in{'action'};
my $notice='';
if ($in{'action'}) {
	&sth_require_post();
 my $id=$in{'queue_id'}||'';
 if ($id !~ /^[A-F0-9]{10}$/ || !$in{'confirm'}) { $notice='Queue action requires a valid ID and explicit confirmation'; }
 else {
   my %cmd=(retry=>['/usr/sbin/postsuper','-r',$id],release=>['/usr/sbin/postsuper','-H',$id],delete=>['/usr/sbin/postsuper','-d',$id]);
   if (!$cmd{$in{'action'}}) { $notice='Unknown action'; }
   else { my ($rc,$out)=&sth_capture(@{$cmd{$in{'action'}}}); $notice=$rc?'Queue action failed':'Queue action submitted'; }
 }
}
my ($rc,$queue)=&sth_capture_limited(65536,'/usr/sbin/postqueue','-p');
&ui_print_header(undef,'Postfix queue','');
print '<p>'.&sth_escape($notice).'</p>' if $notice;
print '<pre>'.&sth_escape($queue).'</pre>';
if ($access{'queue_modify'}) {
 print &ui_form_start('queue.cgi','post');
 print 'Queue ID <input name="queue_id"> <select name="action"><option value="retry">Retry</option><option value="release">Release hold</option><option value="delete">Delete</option></select> ';
 print '<label><input type="checkbox" name="confirm" value="1" required> I confirm this queue action</label><input type="submit" value="Apply"></form>';
}
&ui_print_footer('index.cgi');
