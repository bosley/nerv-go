# nerv-go

### Nerv

Nerv is a simple pub/sub eventing engine for applications that
want to to process events in parallel. 

Nerv offers the ability to spin-up an http endpoint for event submissions, and
has a set of client-side functions that can be used to submit data to a nerv instance.

This project is meant to be used as a library to extend applications but it does offer a CLI server
found in `/cli` that can be used as an example of how to use nerv, as-well-as being used for
testing nerv-based application

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

### Posting Event

```
./nerv -emit -topic "some.topic" -prod "my.producer.id" -data "some fancy string data"
```

