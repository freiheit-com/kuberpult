
## Apps

### Concept

An "App" is essentially a microservice.
* A Kuberpult App contains the microservice **on all environments**.
* An Argo CD App contains the microservice **on one environment**.

Given `n` apps in kuberpult, and `m` environments, there will be `n*m` apps in Argo CD.

The practical limit for number of Argo CD Apps is a few thousand.

### Alternative Names
* Service
* Microservice
