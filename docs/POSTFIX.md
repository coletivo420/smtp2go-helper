# Postfix

The supported pipe service is:

```postfix
smtp2go-helper unix - n n - 1 pipe
  flags=q user=smtp2go-helper:smtp2go-helper null_sender= argv=/usr/local/libexec/smtp2go-helper ${sender} ${recipient} ${queue_id}
```

Postfix 3.10's `pipe(8)` documents `q` as quoting sender/recipient command-line local parts, and `null_sender=` as preserving the empty sender for DSNs. The program receives exactly one recipient because `smtp2go-helper_destination_recipient_limit=1`. No `X` flag is used: SMTP2GO acceptance means relayed to the provider, not proof of final mailbox delivery.

Required routing is `default_transport=smtp2go-helper:`, with `mydestination=localhost`, `local_transport=local:$myhostname`, and `relay_transport=error`. Local destinations (`localhost` and local aliases resolving there) use `local_transport`; normal non-local Internet recipients use `default_transport`. The installer and doctor verify the transport exists before use. `inet_interfaces=loopback-only` keeps SMTP submission private.

The config examples do not set `relayhost`, client SASL, SMTP AUTH, `transport_maps`, or any incoming mailbox/virtual domain infrastructure. Check the actual Postfix documentation installed on the target before adapting the pipe syntax.
