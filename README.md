This is a simple cli tool written in go that helps manage configurations and
passwords for ssh tunnels.  The configurations are stored in a `config.yaml`
file and the passwords are stored in the cli tool pass.  

commands:
connect to db01
```
./tunnels db01
```

set password for this environment
connect to db01
```
./tunnels -p newpassword db01
```
