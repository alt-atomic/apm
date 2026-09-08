# libapm

`libapm` is an asynchronous Vala client for the APM2 D-Bus API. The public
package is `libapm-0.6`, and the introspection namespace is `Apm-0.6`.

The system service provides Packages, Applications, Image, Kernel and Repo.
Distrobox and Icons are provided on the session bus. Create one client for
each bus your application needs:

```vala
async void update_packages () throws Error {
    var apm = yield Apm.Client.connect_bus (BusType.SYSTEM);

    var result = yield apm.packages.update (
        new Apm.Models.UpdateOptions () {
            only_db = false
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

Models use `Serialize.Array` and `Serialize.Dict`. Serialization and
deserialization run outside the calling main context and preserve the field
names used by the current APM2 JSON protocol.
