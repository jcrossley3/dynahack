# dynahack

Just a scratch app to demonstrate how to dynamically apply a
collection of k8s resources from a single yaml file.

    $ dep ensure
    $ go build -v
    $ ./dynahack
    $ ./dynahack knative-serving-0.3.0.yaml
    $ ./dynahack knative-serving-0.3.0.yaml create
    $ ./dynahack knative-serving-0.3.0.yaml get
    $ ./dynahack knative-serving-0.3.0.yaml delete
