#!/usr/bin/perl
use strict;
use warnings;
require './smtp2go-helper-lib.pl';
our (%config);
&sth_init(); &sth_require('view');
my ($rc,$out)=&sth_capture('/usr/bin/journalctl','-u','postfix','--since','-2 hours','--no-pager','-o','cat');
$out='' if $rc;
my @lines=split /\n/,$out;
@lines=@lines[-100..-1] if @lines>100;
$out=join("\n",@lines);
$out =~ s/\r|\n|\t/ /g;
$out =~ s/api-[A-Za-z0-9]{32}/[redacted]/g;
if (length($out)>30000) { $out=substr($out,-30000); }
&ui_print_header(undef,'Recent sanitized logs','');
print '<pre>'.&sth_escape($out).'</pre>';
&ui_print_footer('index.cgi');
