# gosh - a tiny hook server

gosh exists to run shell scripts on demand within a fairly small set
of constraints:

1. The shell scripts must be known ahead of time.
2. No two scripts may run concurrently.
3. Any script will be started at least once after any given trigger.

## Usage

Firstly, gosh runs scripts that are pre-defined in a scripts
directory.  This directory is specified by the final argument to
gosh.  e.g.:

    gosh /some/directory

Assuming that directory has a script called `foo`, you can invoke this
with the following example command:

    curl http://localhost:8888/foo

This will queue an execution and return an HTTP `202`.  This is pretty
much the only status that will ever be returned.

If you request a
path that doesn't match a file, you'll get a `202`.

If the task is already running, another run will be queued.

If the task is already running *and* queued, nothing will happen (a
new run will start after the current request anyway).
