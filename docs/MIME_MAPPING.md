# MIME mapping

`net/mail`, `mime`, `mime/multipart`, `mime/quotedprintable`, `encoding/base64`, and `golang.org/x/net/html/charset` parse RFC 5322/MIME. Multipart recursion is bounded to 30 levels; the full Postfix input and each part are bounded by configured message size.

| MIME element | SMTP2GO `/email/send` |
| --- | --- |
| first valid From, else envelope sender, else configured sender | `sender` |
| current Postfix envelope recipient only | singleton `to` |
| decoded RFC 2047 Subject | `subject` |
| text/plain | `text_body` |
| text/html | `html_body` |
| attachment bytes | `attachments[].fileblob` standard Base64 |
| inline non-text part with Content-ID | `inlines[]`, filename is the Content-ID without angle brackets |
| Reply-To, Message-ID, References and safe list/auto headers | `custom_headers` allowlist |
| To/Cc/Bcc, DKIM-Signature, Received and MIME structure headers | not forwarded as custom headers |

Charsets are converted to UTF-8 via `x/net/html/charset`; quoted-printable and Base64 transfer encoding are decoded before text/attachment mapping. RFC2231 filenames and RFC2047 encoded names are decoded. Malformed transfer encoding or unsupported charset returns a permanent message-data error. Nested multipart entities are traversed recursively. HTML is not converted to plain text. Original recipient headers do not add recipients, preventing duplicate delivery under recipient limit 1.

SMTP2GO supports inline images referenced as `cid:filename`. Its upstream Go struct has no separate Content-ID field, so the MIME Content-ID is used as the inline object's filename identifier. This mapping is covered by unit tests; validate any unusual CID filenames with SMTP2GO before deploying them broadly.
