# Q&A

## Why I cannot start the daemon on Windows?

### Err: panic: failed to listen on any addresses ...

Maybe you are using hyper-v (or docker4win) and it excluded the ports we need to listen

**Solution**:

- run

```powershell
net stop winnat
netsh interface ipv4 show excludedportrange protocol=tcp
netsh interface ipv6 show excludedportrange protocol=tcp

# if the ipv4 havent exclude 52520-52619 
netsh int ipv4 add excludedportrange protocol=tcp startport=52520 numberofports=100

# if the ipv6 havent exclude 52520-52619
netsh int ipv6 add excludedportrange protocol=tcp startport=52520 numberofports=100

net start winnet
```

