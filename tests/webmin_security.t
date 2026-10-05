#!/usr/bin/perl
use strict;
use warnings;
use Test::More;

require './webmin/smtp2go-helper/smtp2go-helper-lib.pl';

my $key = 'api-01234567890123456789012345678901';
my $blob = 'A' x 120;
my $raw = "X-Smtp2go-Api-Key: $key\r\n\"mime_email\": \"$blob\"\nbody data";
my $safe = sth_sanitize_log($raw);
unlike($safe, qr/\Q$key\E/, 'API key is removed');
unlike($safe, qr/\Q$blob\E/, 'long base64-like data is removed');
unlike($safe, qr/mime_email/, 'MIME payload field name is removed');
unlike($safe, qr/[\r\n\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/, 'controls are removed');
cmp_ok(length($safe), '<=', 500, 'log result is bounded');

done_testing();
