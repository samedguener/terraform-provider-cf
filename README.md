Cloud Foundry Terraform Provider [![Build Status](https://travis-ci.org/mevansam/terraform-provider-cf.svg?branch=master)](https://travis-ci.org/mevansam/terraform-provider-cf)
================================

About the fork
--------
This fork contains fixes and features, which were not implemented into the upstream repository.

- [Asynchronous provisioning/deprovisioning and update of cf_service_instance](https://github.com/mevansam/terraform-provider-cf/issues/51)
- [Application Binding to Service Instances fail](https://github.com/mevansam/terraform-provider-cf/issues/40)
- [Deletion of Service Instances](https://github.com/mevansam/terraform-provider-cf/issues/36)
- [Application Update fails](https://github.com/mevansam/terraform-provider-cf/issues/32)
- [Support docker-based Applications](https://github.com/mevansam/terraform-provider-cf/pull/43)
- [Blue/Green-Deployment](https://github.com/mevansam/terraform-provider-cf/pull/22)

Overview
--------

This Terraform provider plugin allows you to configure a Cloud Foundry environment declaratively using [HCL](https://github.com/hashicorp/hcl). The online documentation for the Terraform Cloud Foundry resource is available on the [wiki](https://github.com/mevansam/terraform-provider-cf/wiki).

Requirements
------------

-	[Terraform](https://www.terraform.io/downloads.html) 0.11.x
-	[Go](https://golang.org/doc/install) 1.9 (to build the provider plugin)

Building The Provider
---------------------

Clone repository to: `$GOPATH/src/github.com/terraform-providers/terraform-provider-cf`

```sh
$ mkdir -p $GOPATH/src/github.com/terraform-providers; cd $GOPATH/src/github.com/terraform-providers
$ git clone git@github.com:terraform-providers/terraform-provider-cf
```

Enter the provider directory and build the provider

```sh
$ cd $GOPATH/src/github.com/terraform-providers/terraform-provider-cf
$ make build
```

Using the provider
------------------

Download the release binary and copy it to the `$HOME/terraform.d/plugins/<os>_<arch>/` folder. For example `/home/youruser/terraform.d/plugins/linux_amd64` for a Linux environment or `/Users/youruser/terraform.d/plugins/darwin_amd64` for a MacOS environment.

Developing the Provider
-----------------------

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (version 1.8+ is *required*). You'll also need to correctly setup a [GOPATH](http://golang.org/doc/code.html#GOPATH), as well as adding `$GOPATH/bin` to your `$PATH`.

Clone this repository to `GOPATH/src/github.com/terraform-providers/terraform-provider-cf` as its packaging structure 
has been defined such that it will be compatible with the Terraform provider plugin framwork in 0.10.x.

To compile the provider, run `make build`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

```sh
$ make build
...
$ $GOPATH/bin/terraform-provider-cf
...
```


Testing the Provider
--------------------

To test the provider you will need to run a local PCF Dev instance or launch it in AWS via the `scripts/pcfdev-up.sh`. Once the instance is running you will need to export the following environment variables.

```
export CF_API_URL=https://api.local.pcfdev.io
export CF_USER=admin
export CF_PASSWORD=admin
export CF_UAA_CLIENT_ID=admin
export CF_UAA_CLIENT_SECRET=admin-client-secret
export CF_CA_CERT=""
export CF_SKIP_SSL_VALIDATION=true
```

You can export the following environment variables to enable detail debug logs.

```
export CF_DEBUG=true
export CF_TRACE=debug.log
```

In order to run the tests locally, run.

```
cd cloudfoundry
TF_ACC=1 go test -v -timeout 120m .
```

To run the tests in AWS first launch PCFDev in AWS via `scripts/pcfdev-up.sh`, and then run.

```
make testacc
```

>> Acceptance tests are run against a PCF Dev instance in AWS before a release is created. Any other testing should be done using a local PCF Dev instance. 

```sh
$ make testacc
```

Terraform Links
---------------

- Website: https://www.terraform.io
- [![Gitter chat](https://badges.gitter.im/hashicorp-terraform/Lobby.png)](https://gitter.im/hashicorp-terraform/Lobby)
- Mailing list: [Google Groups](http://groups.google.com/group/terraform-tool)
