# libapm

`libapm` is an asynchronous Vala client for the APM2 D-Bus API. The public
package is `libapm-0.7`, and the introspection namespace is `Apm-0.7`.

The system service provides Packages, Applications, Image, Kernel and Repo.
Distrobox and Icons are provided on the session bus. Create one client for
each bus your application needs:

```vala
async void refresh_package_database () throws Error {
    var apm = yield Apm.Client.connect_bus (BusType.SYSTEM);

    var result = yield apm.packages.update (
        new Apm.ModelsUpdateOptions () {
            only_db = true
        },
        (progress) => {
            message ("%s: %.0f%%", progress.message, progress.percent);
        }
    );

    message (result.message);
}

async void list_containers () throws Error {
    var apm = yield Apm.Client.connect_bus (BusType.SESSION);
    var result = yield apm.distrobox.container_list ();

    foreach (var container in result.containers) {
        message (container.name);
    }
}
```

Methods backed by APM2 jobs wait inside the library until the operation
finishes. Their result is the deserialized operation result; job identifiers
and the Jobs interface are not part of the public API. An optional progress
callback reports job progress without exposing the identifier.

Only operations which APM2 can cancel accept `GLib.Cancellable`. Cancelling
one of these calls sends `Jobs.Cancel` and keeps waiting until the server
confirms a terminal cancelled state. Non-cancellable server operations do not
accept a cancellable argument.

Models use `Deser.DataObject`, `GLib.GenericArray` and `GLib.HashTable` from
`libdeser-0.1`. JSON parsing and serialization run outside the calling main
context; libdeser maps parsed values to models on the calling context. Wire
field names follow the current APM2 JSON protocol.


## libdeser migration

The library depends on `libdeser-0.1`; consumers must rebuild. Public collections
are `GLib.GenericArray<T>` and `GLib.HashTable<string,T>` instead of the previous
Serialize containers. Models inherit from `Deser.DataObject`. Each collection
property supplies its element codec, including nested maps and dynamic values.

Wire naming uses libdeser's default kebab case. Explicit `Description` nicks
remain only where the server requires a different spelling (for example,
`packageInfo` and AppStream's `metadata_license`). Default-valued properties may
be emitted by libdeser; the APM server accepts their explicit false/null values.

APT configuration is request-scoped: set `apt_config` on the operation options.
The removed `apt_config()`/`set_apt_config()` methods no longer exist on the server.
`check_install`, `check_upgrade` and `reinstall` now send their options JSON using
the current server signatures.

## Tests

```sh
meson setup build
meson test -C build --print-errorlogs
```

By default only `libapm-models` is built and registered; it checks wire fixtures,
nested collections and configuration polymorphism. RPM builds use this default.
D-Bus integration tests are enabled explicitly with `-Dintegration_tests=true`
by the E2E script. `libapm-jobs` checks early completion and cancellation.
`libapm-api` calls every public module through an isolated D-Bus service and
checks arguments, typed responses and error propagation.

Run `scripts/test-dbus-e2e.sh` for real service tests in a disposable container.
The script builds and installs APM and libapm, runs the model and isolated-bus
suites, then exercises the public library as root and as an unprivileged user.
It verifies queries, hello install/reinstall/remove transactions, returned job
results, RPM state, remote errors, repository responses and AppStream data.
The existing independent Go D-Bus tests run afterwards. Integration targets
are absent from ordinary builds, and `libapm-live` also requires the prepared
container environment; session-only Distrobox/Icons and platform-only
Image/Kernel operations are covered by the isolated-bus suite.

The current libdeser integer representation limits `uint64` JSON values to
`int64.MAX`; values above that boundary fail explicitly instead of truncating.
APM's native string arrays preserve `null` as distinct from an empty array via
a shared property codec, since libdeser's standard string-list codec rejects null.
