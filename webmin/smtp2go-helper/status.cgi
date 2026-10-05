#!/usr/bin/perl
use strict;
use warnings;
require './smtp2go-helper-lib.pl';
our (%text);
&sth_init(); &sth_require('view');
my $s=&sth_status();
&ui_print_header(undef,'SMTP2GO Helper status','');
print '<pre>';
for my $k (sort keys %$s) { print &sth_escape($k).': '.&sth_escape($s->{$k})."\n"; }
print '</pre>';
&ui_print_footer('index.cgi');
