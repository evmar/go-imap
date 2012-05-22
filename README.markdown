Here's a little IMAP client in Go.

* `src/imap` contains the `imap` package that implements the IMAP protocol client
* `src/imapsync` contains a `main` package that uses the `imap` library to list or download gmail labels (= inboxes)

This hasn't been tested against anything but gmail's IMAP yet, and will likely eat your mail, etc.

To build, set your GOPATH to the base directory and run `go install imapsync`, also from the base directory. This will compile the `imap` library (which will go in `pkg`) and compile and link the `imapsync` test app (which will go in `bin`). 

To test the app, create a file called `auth` that contains your GMail username and password on one line each, and run either `bin/imapsync list` or `bin/imapsync fetch MAILBOXNAME`
