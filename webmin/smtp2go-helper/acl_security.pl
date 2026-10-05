use strict;
use warnings;
require './smtp2go-helper-lib.pl';
our (%text,%in);
sub acl_security_form {
 my ($o)=@_;
 for my $right (qw(view configure replace_api_key test queue_view queue_modify reload_postfix)) {
  print &ui_table_row($right,&ui_yesno_radio($right,$o->{$right}));
 }
}
sub acl_security_save {
 my ($o)=@_;
 for my $right (qw(view configure replace_api_key test queue_view queue_modify reload_postfix)) { $o->{$right}=$in{$right}; }
}
1;
