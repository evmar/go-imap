/*
Package imap implements an IMAP (RFC 3501) client.

Most of the API is straightforward synchronous operations.  See RFC
3501 for a description of the inputs and outputs of these calls.

But IMAP has some tricky details due to its asynchony.  To understand
this, you must know a bit more about the protocol.  In principle, IMAP
allows for multiple outstanding requests in parallel; requests are
tagged with an ID and the success/fail response is tagged with the
same ID.  However, all other data beyond success/fail is sent without
a tag.

Additionally, untagged data may be sent by the server at any time,
even when unrequested.  The RFC says: "A client MUST be prepared to
accept any server response at all times.  This includes server data
that was not requested.  Server data SHOULD be recorded, so that the
client can reference its recorded copy rather than sending a command
to the server to request the data.  In the case of certain server
data, the data MUST be recorded."

This suggests one possible implementation strategy: a Go channel that
provides all incoming untagged data.  However, that API is
unsatisfactory to use.  Many requests, like "list all mailboxes", have
an obvious answer (a []string of mailboxes).  Instead, this library
has a notion of "the current request", and data that arrives in
response to the current request is associated with that request.
Extra data that was unexpected is sent via a separate channel of
untagged data.

This means that in practice code like this will do what you want:

 lists, err := im.List(...)  // lists is now a list of all mailboxes

Except that you must remember to either poll or have a goroutine
reading from the Unsolicited channel for any extra unsolicited data.

Because of this "current request" concept, this library does not
support multiple parallel outstanding requests.  (If you had two
outstanding "list mailboxes" requests, by the IMAP protocol there's no
way to determine which data is in response to which request.)

This is not a large loss: the RFC has confusing language about how
clients MUST NOT send commands that result in ambiguity, without
specifically defining what ambiguitity is (instead giving some
examples).  It seems better left avoided, especially because the
primary interesting operation (fetching messages) allows you to
request multiple messages in a single call and stream in the results.

*/
package imap
