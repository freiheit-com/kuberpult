# For Developers

There are no major design decisions to be discussed, as the CLI is a simple command line tool design to replace
curl requests to the REST endpoints that Kuberpult offers.

The CLI is not supposed to contain any kuberpult related business logic. As such, if you wish to perform some task 
that cannot ultimately be done through one curl request to Kuberpult, you should use the various commands the CLI already 
offers inside a script that performs your intended action.

## Extending the CLI

CLI extension should be made through adding new commands.

The CLI commands follow three major steps:

1. Process user input
   * The CLI processes the parameters provided by the user for some 
   command. These inputs should be validated.
2. Execute request
   * The CLI performs the http request to one of the endpoints provided by Kuberpult, depending on the issued command.
   Every http requests is retried a configurable amount of times.
3. Display response
   * The CLI captures Kuberpult response and displays it. 

When adding a new command, you will be performing steps 1 and some part of 2. Please refer to the implementation of other
commands when implementing your own.
The CLI already offers a standardized way of issuing http requests that deals with timeouts, retries and displaying 
Kuberpults response.

As such, your only job should be to construct a well-formed http request based on your command and the inputs provided by the user.

## On the -use_dex_auth flag...

Some commands offer a -use_dex_auth flag, while others don't.

For endpoints to create releases or conduct release trains, Kuberpult currently offers two REST paths to perform these operations,
one that follows the /api/ path and one that does not. The **use_dex_auth** flag simply dictates what endpoint is used.

The non-/api endpoints will soon be deprecated, as we are currently migrating all of them to the /api endpoints.