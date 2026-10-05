#!/usr/bin/perl
use strict;
use warnings;
sub is_installed {
 return -x '/usr/sbin/postfix' && -x '/usr/local/libexec/smtp2go-helper';
}
1;
