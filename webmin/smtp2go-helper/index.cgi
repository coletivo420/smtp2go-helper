#!/usr/bin/perl
use strict;
use warnings;
require './smtp2go-helper-lib.pl';
our (%access, %text, $module_name);
&sth_init();
&sth_require('view');
my $s=&sth_status();
&ui_print_header(undef,'SMTP2GO Helper','');
print '<h2>SMTP2GO Helper</h2><table class="table">';
for my $pair (
 ['Version',$s->{version}],['Postfix',$s->{postfix}],['Default transport',$s->{transport}],
 ['Recipient limit',$s->{recipient_limit}],['Endpoint',$s->{endpoint}],['Sender domain',$s->{sender_domain}],
 ['API key',$s->{key}],['Key fingerprint',$s->{key_fingerprint}],
 ['/email/send permission',$s->{permission}],['Default sender',$s->{sender}],
 ['Queue active',$s->{queue_active}],['Queue deferred',$s->{queue_deferred}],['Queue hold',$s->{queue_hold}],
 ['Listener',$s->{listener}],['Last helper result',$s->{last_result}]
) { print '<tr><th>'.&sth_escape($pair->[0]).'</th><td>'.&sth_escape($pair->[1]).'</td></tr>'; }
print '</table><ul>';
print '<li><a href="status.cgi">Status / API permissions</a></li>';
print '<li><a href="config.cgi">Configuration</a></li>' if $access{'configure'};
print '<li><a href="test.cgi">Send test via sendmail</a></li>' if $access{'test'};
print '<li><a href="queue.cgi">Postfix queue</a></li>' if $access{'queue_view'};
print '<li><a href="reload.cgi">Reload Postfix</a></li>' if $access{'reload_postfix'};
print '<li><a href="log.cgi">Recent sanitized logs</a></li>';
print '</ul>';
&ui_print_footer('/');
