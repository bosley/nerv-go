# nerv-go

Usage:

```
    -rti        path            absolute path to nerv file to use for command
    -up         <NONE>          start a server
    -down       <NONE>          stop running server
    -force      <NONE>          force kills nerv instance when used in conjunction with -down
    -clean      <NONE>          cleans up potential cruft left after a force kill
    -filter     <NONE>          disable unregistered producers from submitting to endpoints
    -grace      N               set number of seconds to permit when attempting graceful shutdown
    -address    port:ip         set the address/port to bind server to
```

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

```

### Monitoring a topic

```

```
