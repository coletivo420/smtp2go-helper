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

my $api_key = 'api-12345678901234567890123456789012';
my @log_lines = map { sprintf('safe line %03d %s',$_,'x' x 90) } 0..149;
$log_lines[-1] .= " X-Smtp2go-Api-Key: $api_key";
my $window = sth_recent_log_window(join("\n",@log_lines),100,30000);
cmp_ok(length($window), '>', 500, 'log page retains more than status-sized output');
cmp_ok(length($window), '<=', 30000, 'log page output remains bounded');
like($window, qr/safe line 149/, 'newest log lines are preserved');
unlike($window, qr/safe line 000/, 'older lines outside the window are omitted');
unlike($window, qr/\Q$api_key\E/, 'API key is removed from large log window');
cmp_ok(length(sth_sanitize_status('s' x 1000)), '<=', 500, 'status sanitizer retains short limit');

my $queue='-Queue ID- --Size-- ----Arrival Time---- -Sender/Recipient-------' . "\n";
my $count_each=5000;
for my $i (1..($count_each*3)) {
    my $marker = $i % 3 == 0 ? '!' : $i % 3 == 1 ? '*' : '';
    $queue .= sprintf("%010X%s 12 Mon Oct 05 12:00:00 sender\n",$i,$marker);
}
cmp_ok(length($queue), '>', 262144, 'synthetic queue exceeds previous capture limit');
open(my $queue_fh,'<',\$queue) or die "open scalar queue: $!";
my @counts=sth_count_queue_stream($queue_fh);
is_deeply(\@counts,[$count_each,$count_each,$count_each],'streaming queue parser counts all entries');
my @failed=sth_queue_with_command('/bin/false');
is_deeply(\@failed,['unknown','unknown','unknown'],'failed queue command reports unknown counts');

done_testing();
