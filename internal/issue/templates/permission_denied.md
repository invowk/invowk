
# Permission denied!

You don't have permission to perform this operation.

## Common causes:
- Trying to write to a protected directory
- Script file is not executable
- Container engine requires elevated permissions

## Things you can try:
- Check file/directory permissions
- For containers, ensure you're in the docker/podman group:
~~~
$ sudo usermod -aG docker $USER
~~~

- Use rootless containers with Podman
- Run invowk from a directory you own