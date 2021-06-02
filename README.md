# k8snodemon
k8s node monitoring is a simple tool to monitor health of k8s nodes.
If nodes are in non-active state it will reboot them accordingly.

### Service management
We may either obtain token for openstack or create new application credentials.
Below you can find all necessary commands to perform these actions.

```
# create new openstack token
openstack token issue

# create application credentials
openstack application credential create k8snodemon

# list existing application credentials
openstack application credential listopenstack application credential list

# get details about concrete application
openstack application credential show k8snodemon
```
