go-push-mail-proxy
==================

go-push-mail-proxy

To build:
```
  cd go-push-mail-proxy
  GOPATH=$PWD go build mozilla.org/mail-proxy
```

To run:
```
  cp config-example.json config.json
  # ... edit config.json if required
  ./mail-proxy
```

To test:
```
curl -i -X POST -d  '{"username":"foo", "password":"bar", "onNewMessageURL":"http://", "onReconnectURL":"http://"}' localhost:8080/register
