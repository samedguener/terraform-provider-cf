This is a subset of https://github.com/cloudfoundry/cf-acceptance-tests . If this is needed to be included in the vendor directory, we recommend to run the following commands:

```
go get -d github.com/cloudfoundry/cf-acceptance-tests
govendor fetch github.com/cloudfoundry/cf-acceptance-tests/assets/service_broker
```

We omitted these steps to make the tests faster.

The java test has been compiled from https://github.com/mevansam/spring-music , which is a fork of https://github.com/cloudfoundry-samples/spring-music .
