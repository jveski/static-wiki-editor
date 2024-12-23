# Static Wiki Editor

Edit static sites without touching markdown or git!

- ⚡️ Use static site generators like Hugo to build collaborative sites like wikis
- 🤷‍♂️ Make editing easy for non-programmers
- 🌇 Supports image uploads
- 🪶 Built on the flexible [Quill](https://quilljs.com) editor

## Example

![screenshot](./docs/example.png)

## Usage

```shell
podman run -it --rm \
    --workdir /repo \
    -v /opt/my-repo:/repo \
    -p 8080:8080 \
    ghcr.io/jveski/static-wiki-editor:latest --addr=0.0.0.0:8080
```

### Bootstrapping

If the current directory doesn't contain a git repo, the server will clone one from the URL given to `--remote`.

## URL Convention

The server assumes all editable content is stored in the `content` directory relative to the root of the site's git repository i.e. current working directory.
The URL to edit a particular page is prefixed with `/edit` and does not contain the file extension.
So to edit the file `content/foo/bar.md` one would browse to the URL `/edit/foo/bar`.

## Auth

The server expects a trusted reverse proxy (like [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy)) to set `X-Forwarded-Email`.
Requests that do not set an email address will be denied.
All authentication can be disabled by setting `--allow-anonymous`.
