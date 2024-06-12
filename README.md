# nerv-go

## Nerv

Nerv is a simple pub/sub eventing engine for applications that
want to to process events in parallel. It operates on a standard topic/consumer/producer model
where topics can be added, and consumers can subscribe to the topic(s). When an event is
submitted to the engine, the event is given out to consumers based on the configurations of the
respective topic (Broadcast distribution, or Direct distribution with Arbitrary, RoundRobin, and Random selection methods.)

Nerv is meant to be the central driver of an event-driven application, and so, it offers a simple interface to
create "modules" that can be set to start/stop along-with the engine while providing configurations for routing/ forwarding
events.

Within the source code there are two examples of modules being used. 

The first is `module_test` which creates a TCP listener
that forwards `net.conn` objects to the consumers of that module in a round-robin fasion. While "load balancing" this internally
may not make sense, its used strictly for testing.

The second is `modhttp`, a module meant specifically to open the event bus to the world (or local network, etc) via http.
This module contains client and server functions along with an optional "authorization wrapperr + callback" scheme that
can allow a user to filter out any submissions that have invalid or nonexistent API tokens, etc. See `modhttp_test.go` in
the `modhttp` directory.

## The Example

As a means to demonstrate/ test/ and debug nerv instances, the cli in `example` was made. This cli has daemon-like functionality
along with shutdown timers (using the event engine via a `reaper`.) You can use this application to check the status of a nerv instance,
run a nerv instance, submit events, and generally just use it to see what `nerv` is meant for.

Since the example is really the bread and butter of nerv's use-case it is further elaborated on below.

### Starting/ Stopping server instance

Start server at 9092 with specified grace shudown time and server process file.
```
    ./nerv -up -address 127.0.0.1:9092 -grace 8 -rti /tmp/my_rti.json
```

Immediatly kill and re-serve server (note: all registered senders and current messages will be dropped, meaning registered producers will require a re-registration if -filter is enabled.

```
    ./nerv -down -force -clean -up -address 127.0.0.1:9092 -rti /tmp/my_rti.json
```

'force' is required to ensure that the server will come back up immediatly without socket conflicts, which means 'clean' is also required for 'up' to work.

Same things as above, but with defaults:

```
    ./nerv -up
    ./nerv -down -force -clean -up
```

#### Posting Event

```
./nerv -emit -topic "some.topic" -prod "my.producer.id" -data "some fancy string data"
```

#### Optional HTTP request Auth

Handing the http server a function to use and callback-on when a submission request comes in
will enable authentication. Any submission that comes in must include the `Auth` member of
`RequestEventSubmission` found in `modhttp/modhttp.go`. The server and client do not care what form
this authentication is, it simply forwards the data back to the user to see if its valid.

Using api token with CLI:

```
    ./nerv -emit -token "my-special-api-token" -topic "test" -prod "bosley" -data "hello, world!"
```

For examples of usage you can see `main.go` in cli/, or see `modhttp/modhttp_test.go`.
