#include "apt_lock.h"
#include "internal.h"

#include <cstdio>
#include <cstring>
#include <fcntl.h>
#include <unistd.h>

// Probes 'path' with fcntl(F_GETLK) to see if a write lock is held.
static bool check_lock_file(const std::string &path, int &holder_pid) {
    holder_pid = -1;

    const int fd = open(path.c_str(), O_RDWR);
    if (fd < 0) {
        if (errno == ENOENT || errno == EACCES) {
            return true;
        }
        return false;
    }

    flock fl{};
    fl.l_type = F_WRLCK;
    fl.l_whence = SEEK_SET;
    fl.l_start = 0;
    fl.l_len = 0;
    fl.l_pid = 0;

    if (fcntl(fd, F_GETLK, &fl) == -1) {
        close(fd);
        return false;
    }

    close(fd);

    if (fl.l_type == F_UNLCK) {
        return true;
    }

    if (fl.l_pid > 0) {
        holder_pid = fl.l_pid;
    }
    return false;
}

// Reads the process name from /proc/<pid>/comm.
static std::string get_process_name(const int pid) {
    if (pid <= 0) return "";

    char path[64];
    snprintf(path, sizeof(path), "/proc/%d/comm", pid);

    FILE *f = fopen(path, "r");
    if (!f) return "";

    char name[256];
    if (fgets(name, sizeof(name), f)) {
        fclose(f);
        if (const size_t len = strlen(name); len > 0 && name[len - 1] == '\n') {
            name[len - 1] = '\0';
        }
        return name;
    }

    fclose(f);
    return "";
}

// Holds paths to the archives and lists lock files.
struct LockPaths {
    char *archives_lock;
    char *lists_lock;
};

// Builds lock file paths from APT configuration directories.
static LockPaths get_lock_paths() {
    LockPaths paths{};

    const std::string archives_dir = _config->FindDir("Dir::Cache::Archives", "/var/cache/apt/archives/");
    paths.archives_lock = safe_strdup(archives_dir + "lock");

    const std::string lists_dir = _config->FindDir("Dir::State::lists", "/var/lib/apt/lists/");
    paths.lists_lock = safe_strdup(lists_dir + "lock");

    return paths;
}

// Frees strings in `paths` and zeroes the struct.
static void free_lock_paths(LockPaths &paths) {
    free(paths.archives_lock);
    free(paths.lists_lock);
    memset(&paths, 0, sizeof(LockPaths));
}

// Probes archives and lists lock files without blocking.
AptLockStatus apt_check_lock_status() {
    AptLockStatus status{};
    status.is_locked = false;
    status.can_acquire = true;
    status.lock_pid = -1;

    try {
        LockPaths paths = get_lock_paths();

        // Check archives lock (main lock for install/upgrade operations)
        if (int archives_holder_pid = -1; !check_lock_file(paths.archives_lock, archives_holder_pid)) {
            status.is_locked = true;
            status.can_acquire = false;
            status.lock_pid = archives_holder_pid;
            status.lock_file_path = safe_strdup(paths.archives_lock);

            if (archives_holder_pid > 0) {
                if (const std::string proc_name = get_process_name(archives_holder_pid); !proc_name.empty()) {
                    status.lock_holder = safe_strdup(proc_name);
                }
            }

            free_lock_paths(paths);
            return status;
        }

        // Check lists lock (lock for update operations)
        if (int lists_holder_pid = -1; !check_lock_file(paths.lists_lock, lists_holder_pid)) {
            status.is_locked = true;
            status.can_acquire = false;
            status.lock_pid = lists_holder_pid;
            status.lock_file_path = safe_strdup(paths.lists_lock);

            if (lists_holder_pid > 0) {
                if (const std::string proc_name = get_process_name(lists_holder_pid); !proc_name.empty()) {
                    status.lock_holder = safe_strdup(proc_name);
                }
            }

            free_lock_paths(paths);
            return status;
        }

        free_lock_paths(paths);
    } catch (const std::exception &e) {
        status.error_message = safe_strdup(std::string("Exception: ") + e.what());
        status.can_acquire = false;
    }

    return status;
}

// Frees heap strings and zeroes `status`.
void apt_free_lock_status(AptLockStatus *status) {
    if (!status) return;
    free(status->lock_holder);
    free(status->lock_file_path);
    free(status->error_message);
    memset(status, 0, sizeof(AptLockStatus));
}
