
# Dockerfile not found!

The 'container' runtime requires a Dockerfile to build the execution environment.

## Things you can try:
- Create a Dockerfile in the same directory as your invowkfile:
~~~dockerfile
FROM debian:stable-slim
RUN apk add --no-cache bash coreutils
WORKDIR /workspace
~~~

- Or specify a Dockerfile path in your invowkfile:
~~~cue
container: {
  dockerfile: "path/to/Dockerfile"
}
~~~

- Or use a pre-built image:
~~~cue
container: {
  image: "ubuntu:22.04"
}
~~~