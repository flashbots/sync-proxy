#

[![Goreport status](https://goreportcard.com/badge/github.com/flashbots/sync-proxy)](https://goreportcard.com/report/github.com/flashbots/sync-proxy)
[![Test status](https://github.com/flashbots/sync-proxy/workflows/Checks/badge.svg)](https://github.com/flashbots/sync-proxy/actions?query=workflow%3A%22Checks%22)

Flashbots proxy to allow redundant execution client (EL) state sync post merge.

- Runs a proxy server that proxies requests from a beacon node (BN) to multiple other execution clients
- Can drive EL sync from multiple BNs for redundancy

## Getting Started

- Run a BN with the execution endpoint pointing to the proxy (default is `localhost:25590`).
- Start the proxy with a flag specifying one or multiple EL endpoints (make sure to point to the authenticated port).

```bash
git clone https://github.com/flashbots/sync-proxy.git
cd sync-proxy
make build

# Show the help
./sync-proxy -help
```

To run with multiple EL endpoins:

```
./sync-proxy -builders="localhost:8551,localhost:8552"
```

### Nginx

The sync proxy can also be used with nginx, with requests proxied from the beacon node to a local execution client and mirrored to multiple sync proxies.

![nginx setup overview](docs/nginx-setup.png)

An example nginx config like this can be run with the sync proxy:

<details>
<summary><code>/etc/nginx/conf.d/sync_proxy.conf</code></summary>

```ini
server {
        listen 8552;
        listen [::]:8552;

        server_name _;

        location / {
                mirror /sync_proxy_1;
                mirror /sync_proxy_2;

                proxy_pass http://localhost:8551;
                proxy_set_header X-Real-IP $remote_addr;
                proxy_set_header Host $host;
                proxy_set_header Referer $http_referer;
                proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        }

        #
        # execution nodes
        #
        location = /sync_proxy_1 {
                internal;
                proxy_pass http://sync-proxy-1.local:8552$request_uri;
                proxy_connect_timeout 100ms;
                proxy_read_timeout 100ms;
        }

        location = /sync_proxy_2 {
                internal;
                proxy_pass http://sync-proxy-2.local:8552$request_uri;
                proxy_connect_timeout 100ms;
                proxy_read_timeout 100ms;
        }
}
```

</details>

And if you'd like to use different JWT secrets for different ELs:

<details>
<summary>Example</summary>
First, install jwt-tokens-service: `go install github.com/flashbots/sync-proxy/cmd/jwt-tokens-service@latest`

Set up the service, e.g. for systemd:

```
[Unit]
Description=JWT tokens service
After=network.target
Wants=network.target

[Service]
Type=simple

ExecStart=/.../jwt-tokens-service \
    -config /.../jwt-secrets.json \
    -client-id some-cl-name

[Install]
WantedBy=default.target
```

Generate a secret for each EL with `openssl rand -hex 32` and put them in a JSON file:

```json
{
  "sync-proxy-1": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdee",
  "sync-proxy-2": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}
```

Then, set up nginx:

```
# /etc/nginx/conf.d/sync_proxy.conf
server {
        listen 8552;
        listen [::]:8552;

        server_name _;

        location / {
                mirror /sync_proxy_1;
                mirror /sync_proxy_2;

                auth_request /_tokens;
                # make sure to lowercase and replace dashes with underscores from names in json config
                auth_request_set $auth_header_sync_proxy_1 $upstream_http_authorization_sync_proxy_1;
                auth_request_set $auth_header_sync_proxy_2 $upstream_http_authorization_sync_proxy_2;

                proxy_pass http://localhost:8551;
                proxy_set_header X-Real-IP $remote_addr;
                proxy_set_header Host $host;
                proxy_set_header Referer $http_referer;
                proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        }

        location = /_tokens {
                internal;
                proxy_pass http://127.0.0.1:1337/tokens/;
                proxy_pass_request_body off;
                proxy_set_header Content-Length "";
        }

        #
        # execution nodes
        #
        location = /sync_proxy_1 {
                internal;
                proxy_pass http://sync-proxy-1.local:8552$request_uri;
                proxy_connect_timeout 100ms;
                proxy_read_timeout 100ms;

                proxy_hide_header Authorization;
                proxy_set_header Authorization $auth_header_sync_proxy_1;
        }

        location = /sync_proxy_2 {
                internal;
                proxy_pass http://sync-proxy-2.local:8552$request_uri;
                proxy_connect_timeout 100ms;
                proxy_read_timeout 100ms;

                proxy_hide_header Authorization;
                proxy_set_header Authorization $auth_header_sync_proxy_2;
        }
}
```

</details>

## Caveats

The sync proxy attempts to sync to the beacon node with the highest timestamp in the `engine_forkchoiceUpdated` and `engine_newPayload` calls and forwards to the execution clients.

The sync proxy also attempts to identify the best beacon node based on the originating host of the request. If you are using the same host for multiple beacon nodes to sync the EL, the sync proxy won't be able to distinguish between the beacon nodes and will proxy all requests from the same host to the configured ELs.
