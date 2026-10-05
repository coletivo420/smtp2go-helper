#!/usr/bin/perl
use strict;
use warnings;
require './smtp2go-helper-lib.pl';
our (%config);
&sth_init(); &sth_require('view');
my ($rc,$out)=&sth_capture_limited(64000,'/usr/bin/journalctl','-u','postfix','--since','-2 hours','--no-pager','-o','cat');
$out='' if $rc;
my @lines=split /\n/,$out;
@lines=@lines[-100..-1] if @lines>100;
$out=join("\n",@lines);
$out=&sth_sanitize_log($out);
if (length($out)>30000) { $out=substr($out,-30000); }
&ui_print_header(undef,'Recent sanitized logs','');
print '<pre>'.&sth_escape($out).'</pre>';
&ui_print_footer('index.cgi');
