# Keyless SSL for any IP address

`keyless` is a Go package (and a server-side component)
allowing you to run an embed HTTPS server on any IP address,
and have it be trusted by every browser that trusts Let's Encrypt certificates.

Imagine you're building some software that includes an embed HTTPS server,
which you want to distribute to your users.
Your users run the software and access it from their browsers,
typically on localhost, but often on a link-local IP address.
Increasingly, they'll get warnings that this is not secure,
and some HTTPS-only web features will be disabled.

`keyless` helps you to obtain a certificate for a domain that can resolve to any IP address,
including localhost and link-local addresses.

See [caveats](#caveats)!

The Go package is quite simple to use:

```go
srv := http.Server{
	TLSConfig: &tls.Config{
		GetCertificate: keyless.GetCertificate("keyless.example.com"),
	},
}

srv.ListenAndServeTLS("", "")
```

This runs an HTTPS server that gets its certificate dynamically from a server running on `keyless.example.com`.
This is where all the magic happens.

## Keyless server

The `keyless` package depends on a server-side component, `keyless-server`,
that expects to run under `systemd` on Linux.

It includes two components:
- a DNS server that resolves names like `192-168-1-1.ip.example.com` to `192.168.1.1`; and
- an HTTPS server that gives your server access to a certificate for `*.ip.example.com`.

What separates this from other similar systems,
is that the private key for the `*.ip.example.com` certificate
is never distributed by `keyless-server`, thus complying with Let's Encrypt's ToS.

### Setup

To setup `keyless-server` you need a domain name (`example.com`) and a Linux server.

In your **nameserver**, create:
- records for `keyless.example.com` pointing to your server (`A`/`AAAA` or `CNAME`); and
- an `NS` record for `ip.example.com` pointing to `keyless.example.com`.

Download and build `keyless-server`, and create a `config.json` next to it:

```json
{
    "domain":          "ip.example.com",
    "nameserver":      "keyless.example.com",

    "certificate":     "certificates/cert.pem",
    "master_key":      "certificates/master.pem",

    "api": {
        "handler":     "keyless.example.com/",
        "certificate": "api/cert.pem",
        "key":         "api/key.pem",
    },

    "letsencrypt": {
        "account":     "letsencrypt/account.json",
        "account_key": "letsencrypt/account.pem"
    }
}
```

Then, run `./keyless-server setup`.<br>
Make sure it can bind to ports 53 and 443 (perhaps by using `sudo`).

The `setup` will guide you through the process of creating a Let's Encrypt account
(do not use the production API for testing!), private keys,
a certificate for `*.ip.example.com`, and another for `keyless.example.com`.

After this is done you should configure `systemd` to run `keyless-server`.

For example this is `keyless.service`:
```ini
[Unit]
Description=keyless
Requires=network.target

[Service]
Type=notify
Restart=on-failure
ExecStart=/home/keyless/keyless-server
WorkingDirectory=/home/keyless
User=keyless
NonBlocking=true

[Install]
WantedBy=multi-user.target
```

And this is `keyless.socket`:
```ini
[Unit]
Description=keyless socket

[Socket]
ListenStream=443
ListenDatagram=53

[Install]
WantedBy=sockets.target
```

## Caveats

This project is quite young and instructions terse.
**This is deliberate!**

Please, familiarize yourself with the code, what it's doing.
If you do use it, **take ownership.**

I'm using this myself (for [RethinkRAW](https://rethinkraw.com)), making it as I go, and I decided to share it.<br>
I'm not claiming this is production ready, or even a particularly good idea,
although I'm not the first one to do something similar (see [localtls](https://github.com/Corollarium/localtls)).

The biggest claim behind this project, and the reason it's named `keyless`
is that the private key never leaves `keyless-server`.
The way that works is basically how [Keyless SSL](https://blog.cloudflare.com/keyless-ssl-the-nitty-gritty-technical-details/) works.

This is important to ensure we abide by the Let's Encrypt ToS.
But it ***does not mean these connections are impossible to MitM***.

Because it's using a single wildcard certificate for all domains,
an active attacker can still impersonate any server that is using the same private key.

One possible, supported, mitigation, is to lock the API down with a client certificate,
which you'll need to obfuscate/secure.
This raises the bar, but you're open to reverse engineering.

Another mitigation is to only resolve link-local addresses,
assuming you don't have bad actors on your LAN,
where this is most needed.
