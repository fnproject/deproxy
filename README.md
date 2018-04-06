# Deproxy

`deproxy` is an inline vanity proxy for golang specifically built to work with `dep`.

At time of writing, `dep` does not support private git repos for package resolution [see here](https://github.com/golang/dep/issues/174).  This means that enterprises using private git repos cannot use dep without [vanity package names](https://golang.org/cmd/go/#hdr-Remote_import_paths) - these are themselves often hard to enable in an enterprise environment where builds may be isolated and publishing canonical internal URLs may be hard.

`deproxy` allows one or more project to manage dependencies based on private repositories via `dep` without the need for additional fixed infrastructure.

This does not work for `go get` and its untested on other dependency managers (glide etc).

## Usage

Deproxy works on linux and osx.

Install:
```sh
go get github.com/fnproject/deproxy
```

Deproxy allows you to reference vanity-named packages  (e.g. `mycompany.mydomain/foo/mypackage`) on private repos.

Your code, and all dependencies must reference packages consistently via the same vanity packages, including via transitive dependencies and internally within those packages themselves-this tool *DOES NOT* solve issues around different namespacing.

Domains in vanity packages do not need to be resolvable.

All packages under a given domain must be vanity packages and must be re-written (i.e. you can't mix and match rewritten and non-rewritten URLS on the smae domain and you can't use deproxy to override public github or public bitbucket packages).


`deproxy` uses a rewrite file : `Deproxy.toml` this must contain  `[[ rewrite ]]` entries for each top-level package containing it's corresponding git repo.


```
[[ rewrite ]]
   package = "mydomain.com/projecta/packagea"
   source = "ssh://git@bitbucket.myinternaldomain.com/projecta/packagea.git"


[[ rewrite ]]
   package = "mydomain.com/projectb/packageb"
   source = "ssh://git@internal_git_server:7999/projectb/packageb.git"

```

Where `package` is the *root* package of a dependency (not a sub-package) and `source` is a valid http or ssh git url.

Note if `source` is https it *must* be on a domain that is not included in any `package` reference.

Store this in the root of your repo.

To use deproxy you need to run it as a command wrapper around `dep` e.g.:

```
deproxy dep init
```

```
deproxy dep ensure --update vanity.domain/mypackage
```

set `DEPROXY_VERBOSE=1` to see logs

## Internals

Deproxy runs an intercepting http proxy on a random port and injects this into dep's environment via proxy envirnment variables - deproxy will still use any environment proxies set on invocation as upstream proxies

It intercepts all SSL CONNECT traffic to any host mentioned in the rewrite file and blocks it (this causes `dep` to retry on the HTTP port)  and then serves a vanity redirect on the http version of the URL.


