#!/usr/bin/perl
use strict;
use warnings;
require './smtp2go-helper-lib.pl';
our (%in);
&sth_init();
&sth_require('reload_postfix');
my $message='';
if ($in{'confirm'} && $in{'reload'}) {
 my ($rc,$out)=&sth_capture('/usr/sbin/postfix','check');
 if ($rc) { $message='postfix check failed; reload was not run'; }
 else {
   my ($rrc,$rout)=&sth_capture('/usr/sbin/postfix','reload');
   $message=$rrc?'Postfix reload failed':'Postfix reloaded after a successful check';
 }
}
&ui_print_header(undef,'Reload Postfix','');
print '<p>'.&sth_escape($message).'</p>' if $message;
print &ui_form_start('reload.cgi').'<input type="hidden" name="reload" value="1">';
print '<label><input type="checkbox" name="confirm" value="1" required> I confirm reload after postfix check</label><input type="submit" value="Reload Postfix"></form>';
&ui_print_footer('index.cgi');
