#!/usr/bin/perl
use strict;
use warnings;
require './smtp2go-helper-lib.pl';
our (%config);
&sth_init(); &sth_require('view');
my ($rc,$out)=&sth_capture_tail_limited(64000,'/usr/bin/journalctl','-u','postfix','--since','-2 hours','-n','100','--no-pager','-o','cat');
$out='' if $rc;
$out=&sth_recent_log_window($out,100,30000);
&ui_print_header(undef,'Recent sanitized logs','');
print '<pre>'.&sth_escape($out).'</pre>';
&ui_print_footer('index.cgi');
