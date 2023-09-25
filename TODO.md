TODO
====

Implement the following
-----------------------

* [x] Update CR structure to use several public keys and pod selectors,
  no need to use secrets for the public keys
* [x] Update SSH server to accommodate multiple public keys and pod selectors for
  the login
* [x] Fix the scenario with changing ssh ingress resource - at present it looks like
  it adds a copy
* [x] Update SSH server to pass the command from the command line as entrypoint
  array (command field). If starting a new container it works as 'entrypoint'
  in the container options. But what to do if attached to the existing
  container? Can I specify the command when attaching? seems like yes - kubectl
  exec can do this, and our debug session seems the same.
* [x] Implement debug with command as exec for existing containers
* [x] Add Exec mode
* [x] Fix wired characters displayed in the exec mode. Why attach behave correctly?
* [x] Support namespace-scoped resources
* [x] Support target container from the configuration
* [x] Implement pod and container selection from the command line in the pod@server or pod:container@server
* [x] Fix double Ctrl+C for stopping the controller
* [x] Implement interactive pod and container selection (see also Spec.Containers)
* [x] Delay on incorrect auth
* [x] Tests
* [x] Add a proper global server configuration
* [x] Cleanup
* [x] CI on GitHub
* [ ] Documentation
* [ ] Add username to SSH keys in resources, otherwise it's hard to audit
* [ ] Helm chart?
* [ ] Default image for the debug environment
* [ ] Fix demo scene bug (interactive choice is not necessary when the choice of target container is unambiguous)
* [ ] Propose something for SCP (looks like this is hard enough)

* [ ] Document the situation with RSA signatures for public keys: there is a hack
  to enable it in golang/x/crypto (additional details in
  <https://stackoverflow.com/questions/70291932/ssh-server-in-go-how-to-offer-public-key-types-different-than-rsa>)
  I have a feeling that that was working some time ago...
