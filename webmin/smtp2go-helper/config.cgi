#!/usr/bin/perl
use strict;
use warnings;
require './smtp2go-helper-lib.pl';
our (%in,%config,%access);
&sth_init();
&sth_require('view');
&sth_require('configure') if $in{'save_config'};
&sth_require('replace_api_key') if $in{'replace_key'};
my $path=$config{'config_file'}||'/etc/smtp2go-helper/config.json';
my $loaded_cfg=&sth_read_config();
my $cfg=$loaded_cfg||{};
my $message='';
if ($in{'save_config'}) {
	&sth_require_post();
 if (!defined $loaded_cfg) { $message='Current configuration metadata is unsafe or unreadable'; }
 else {
 my $timeout=$in{'timeout_seconds'}||'';
 my $max=$in{'max_message_bytes'}||'';
 my $fast=$in{'fastaccept'}?JSON::PP::true():JSON::PP::false();
 my $sender=$in{'default_sender'}||'';
 my $level=$in{'log_level'}||'';
 if ($timeout !~ /^\d+$/ || $timeout<1 || $timeout>300 ||
     $max !~ /^\d+$/ || $max<1024 || $max>10240000 ||
     ($sender ne '' && $sender !~ /^[^\s<>\r\n]+\@[^\s<>\r\n]+$/) ||
     $level !~ /^(debug|info|warn|error)$/) {
   $message='Invalid configuration values';
 } else {
   my $new={endpoint=>'https://api.smtp2go.com/v3/email/send',timeout_seconds=>0+$timeout,
     fastaccept=>$fast,default_sender=>$sender,max_message_bytes=>0+$max,log_level=>$level};
   my $old=defined($cfg) ? JSON::PP->new->canonical->pretty->encode($cfg) : undef;
   if (!defined $old) { $message='Cannot read current configuration for safe replacement'; }
   else {
     eval { &sth_write_atomic($path,JSON::PP->new->canonical->pretty->encode($new),0640,(getgrnam('smtp2go-helper'))[2]); };
     if ($@) { $message='Could not write configuration safely'; }
     else {
       my ($rc,$out)=&sth_capture($config{'helper_bin'}||'/usr/local/libexec/smtp2go-helper','config','validate');
       if ($rc) { eval { &sth_write_atomic($path,$old,0640,(getgrnam('smtp2go-helper'))[2]); }; $message='Validation failed; previous configuration restored'; }
       else { $cfg=$new; $message='Configuration saved and validated'; }
     }
   }
 }
 }
}
if ($in{'replace_key'}) {
	&sth_require_post();
 my $key=$in{'new_api_key'}||'';
	$in{'new_api_key'}='';
 $key =~ s/[\r\n]+$//;
 if ($key !~ /^api-[A-Za-z0-9]{32}$/) { $message='API key format invalid'; }
 else {
   my $keypath=$config{'key_file'}||'/etc/smtp2go-helper/api.key';
   my $old=&sth_read_key_secure();
   my $had_old=defined($old) ? 1 : 0;
   my $exists=(-e $keypath || -l $keypath) ? 1 : 0;
   if ($exists && !$had_old) { $message='Existing key file metadata is unsafe; no change made'; }
   else {
     eval { &sth_write_atomic($keypath,$key."\n",0640,(getgrnam('smtp2go-helper'))[2]); };
     if ($@) { $message='Could not replace key safely'; }
     else {
       my ($rc,$out)=&sth_capture($config{'helper_bin'}||'/usr/local/libexec/smtp2go-helper','api','permissions');
       if ($rc || $out !~ m{/email/send:\s*allowed}) {
         if ($had_old) { eval { &sth_write_atomic($keypath,$old."\n",0640,(getgrnam('smtp2go-helper'))[2]); }; }
         else { unlink($keypath); }
         $message='Permission check failed; previous key restored';
       } else { $message='API key replaced; key value is not displayed'; }
     }
   }
   $old="\0" x length($old) if defined $old;
 }
 $key="\0" x length($key);
}
&ui_print_header(undef,'SMTP2GO Helper configuration','');
print '<p>'.&sth_escape($message).'</p>' if $message;
print &ui_form_start('config.cgi','post');
print '<input type="hidden" name="save_config" value="1"><table>';
for my $row (['Timeout seconds','timeout_seconds',30],['Maximum message bytes','max_message_bytes',10240000]) {
 print '<tr><th>'.&sth_escape($row->[0]).'</th><td><input name="'.$row->[1].'" value="'.&sth_escape($cfg->{$row->[1]}//$row->[2]).'"></td></tr>';
}
print '<tr><th>Default sender</th><td><input name="default_sender" value="'.&sth_escape($cfg->{default_sender}//'').'"></td></tr>';
print '<tr><th>Log level</th><td><select name="log_level">';
for my $level (qw(debug info warn error)) { print '<option'.(($cfg->{log_level}//'info') eq $level?' selected':'').'>'.$level.'</option>'; }
print '</select></td></tr>';
print '<tr><th>fastaccept</th><td><input type="checkbox" name="fastaccept" value="1"'.($cfg->{fastaccept}?' checked':'').'></td></tr>';
print '<tr><th>Endpoint</th><td>https://api.smtp2go.com/v3/email/send (fixed)</td></tr></table><input type="submit" value="Save"></form>';
if ($access{'replace_api_key'}) {
 print '<h3>Replace API key</h3>'.&ui_form_start('config.cgi','post');
 print '<input type="hidden" name="replace_key" value="1"><input type="password" name="new_api_key" autocomplete="new-password">';
 print '<input type="submit" value="Replace key"></form>';
}
&ui_print_footer('index.cgi');
