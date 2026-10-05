#!/usr/bin/perl
use strict;
use warnings;
require './smtp2go-helper-lib.pl';
our (%access, %text, $module_name);
&sth_init();
&sth_require('view');
my $s=&sth_status();
&ui_print_header(undef,&sth_t('index_title','SMTP2GO Helper'),'');
print '<h2>'.&sth_escape(&sth_t('index_title','SMTP2GO Helper')).'</h2><table class="table">';
for my $pair (
 [&sth_t('version','Version'),$s->{version}],[&sth_t('postfix','Postfix'),$s->{postfix}],[&sth_t('transport','Default transport'),$s->{transport}],
 [&sth_t('recipient_limit','Recipient limit'),$s->{recipient_limit}],[&sth_t('endpoint','Endpoint'),$s->{endpoint}],[&sth_t('sender_domain','Sender domain'),$s->{sender_domain}],
 [&sth_t('api_key','API key'),$s->{key}],[&sth_t('key_fingerprint','Key fingerprint'),$s->{key_fingerprint}],
 [&sth_t('permission','/email/send permission'),$s->{permission}],[&sth_t('default_sender','Default sender'),$s->{sender}],
 [&sth_t('queue_active','Queue active'),$s->{queue_active}],[&sth_t('queue_deferred','Queue deferred'),$s->{queue_deferred}],[&sth_t('queue_hold','Queue hold'),$s->{queue_hold}],
 [&sth_t('listener','Listener'),$s->{listener}],[&sth_t('last_result','Last helper result'),$s->{last_result}]
) { print '<tr><th>'.&sth_escape($pair->[0]).'</th><td>'.&sth_escape($pair->[1]).'</td></tr>'; }
print '</table><ul>';
print '<li><a href="status.cgi">'.&sth_escape(&sth_t('status_link','Status / API permissions')).'</a></li>';
print '<li><a href="config.cgi">'.&sth_escape(&sth_t('config_link','Configuration')).'</a></li>' if $access{'configure'};
print '<li><a href="test.cgi">'.&sth_escape(&sth_t('test_link','Send test via sendmail')).'</a></li>' if $access{'test'};
print '<li><a href="queue.cgi">'.&sth_escape(&sth_t('queue_link','Postfix queue')).'</a></li>' if $access{'queue_view'};
print '<li><a href="reload.cgi">'.&sth_escape(&sth_t('reload_link','Reload Postfix')).'</a></li>' if $access{'reload_postfix'};
print '<li><a href="log.cgi">'.&sth_escape(&sth_t('logs_link','Recent sanitized logs')).'</a></li>';
print '</ul>';
&ui_print_footer('/');
