#include "apt_logging.h"
#include "progress.h"
#include "error.h"

#include <atomic>
#include <cstdio>
#include <fcntl.h>
#include <sstream>
#include <sys/select.h>
#include <thread>
#include <unistd.h>

AptLogCallback g_log_callback = nullptr;
uintptr_t g_log_user_data = 0;

// Sends `msg` to the log callback, or falls back to stderr.
void emit_log(const std::string &msg) {
    if (g_log_callback) {
        g_log_callback(msg.c_str(), g_log_user_data);
    } else {
        std::cerr << msg << std::endl;
    }
}

// Streambuf that buffers output and forwards complete lines to emit_log().
class EmitLogBuf : public std::streambuf {
public:
    EmitLogBuf() = default;

protected:
    int overflow(const int ch) override {
        if (ch == EOF) return 0;
        if (ch == '\n' || ch == '\r') {
            flush_line();
        } else {
            buffer_.push_back(static_cast<char>(ch));
        }
        return ch;
    }

    int sync() override {
        flush_line();
        return 0;
    }

private:
    void flush_line() {
        if (!buffer_.empty()) {
            emit_log(buffer_);
            buffer_.clear();
        }
    }

    std::string buffer_;
};

static EmitLogBuf g_emit_buf;
static std::ostream g_emit_stream(&g_emit_buf);

std::ostream &apt_log_stream() {
    return g_emit_stream;
}

// RAII wrapper for C stderr pipe capture with a background reader thread.
struct StderrCapture {
    int saved_fd = -1;
    int pipe_read = -1;
    int pipe_write = -1;
    std::string captured;
    std::thread reader;
    std::atomic<bool> running{false};

    bool start() {
        fflush(stderr);
        captured.clear();

        int fds[2];
        if (pipe(fds) != 0) return false;

        pipe_read = fds[0];
        pipe_write = fds[1];
        fcntl(pipe_read, F_SETFL, O_NONBLOCK);
        saved_fd = dup(STDERR_FILENO);
        dup2(pipe_write, STDERR_FILENO);
        running.store(true);
        reader = std::thread(&StderrCapture::read_loop, this);
        return true;
    }

    void stop() {
        fflush(stderr);

        if (saved_fd >= 0) {
            dup2(saved_fd, STDERR_FILENO);
            close(saved_fd);
            saved_fd = -1;
        }
        if (pipe_write >= 0) {
            close(pipe_write);
            pipe_write = -1;
        }
        if (running.load()) {
            running.store(false);
            if (reader.joinable()) reader.join();
        }
        if (pipe_read >= 0) {
            close(pipe_read);
            pipe_read = -1;
        }
    }

    void flush_to_log() const {
        if (captured.empty() || !g_log_callback) return;

        std::istringstream stream(captured);
        std::string line;
        while (std::getline(stream, line)) {
            if (!line.empty()) {
                g_log_callback(line.c_str(), g_log_user_data);
            }
        }
    }

private:
    void read_loop() {
        char buf[1024];
        while (running.load()) {
            fd_set fds;
            FD_ZERO(&fds);
            FD_SET(pipe_read, &fds);
            timeval tv = {0, 100000};
            if (const int ret = select(pipe_read + 1, &fds, nullptr, nullptr, &tv);
                ret > 0 && FD_ISSET(pipe_read, &fds)) {
                if (const ssize_t n = read(pipe_read, buf, sizeof(buf) - 1); n > 0) {
                    buf[n] = '\0';
                    captured += buf;
                }
            }
        }
        // Drain remaining data
        while (true) {
            const ssize_t n = read(pipe_read, buf, sizeof(buf) - 1);
            if (n <= 0) break;
            buf[n] = '\0';
            captured += buf;
        }
    }
};

// C++ iostream redirect state.
struct IoStreamCapture {
    std::streambuf *prev_cout = nullptr;
    std::streambuf *prev_cerr = nullptr;
    std::streambuf *prev_clog = nullptr;
    bool active = false;

    void start() {
        prev_cout = std::cout.rdbuf(g_emit_stream.rdbuf());
        prev_cerr = std::cerr.rdbuf(g_emit_stream.rdbuf());
        prev_clog = std::clog.rdbuf(g_emit_stream.rdbuf());
        active = true;
    }

    void stop() {
        try {
            g_emit_stream.flush();
            std::cout.flush();
            std::cerr.flush();
            std::clog.flush();
        } catch (...) {}

        std::cout.rdbuf(prev_cout);
        std::cerr.rdbuf(prev_cerr);
        std::clog.rdbuf(prev_clog);
        prev_cout = nullptr;
        prev_cerr = nullptr;
        prev_clog = nullptr;
        active = false;
    }
};

static StderrCapture g_stderr_capture;
static IoStreamCapture g_iostream_capture;

// Redirects C++ cout/cerr/clog and C stderr to the log callback.
extern "C" void apt_capture_stdio(const int enable) {
    if (enable && !g_iostream_capture.active) {
        g_iostream_capture.start();
        g_stderr_capture.start();
    } else if (!enable && g_iostream_capture.active) {
        g_iostream_capture.stop();
        g_stderr_capture.stop();
        g_stderr_capture.flush_to_log();
        g_stderr_capture.captured.clear();
    }
}

extern "C" void apt_set_log_callback(const AptLogCallback callback, const uintptr_t user_data) {
    g_log_callback = callback;
    g_log_user_data = user_data;
}

extern "C" void goAptProgressCallback(const char *package_name,
                                      AptCallbackType callback_type,
                                      uint64_t current,
                                      uint64_t total,
                                      uint64_t speed_bps,
                                      uintptr_t user_data);

extern "C" void goAptLogCallback(const char *message, uintptr_t user_data);

// Wires the global progress callback to the Go-side bridge function.
extern "C" void apt_use_go_progress_callback(const uintptr_t user_data) {
    global_callback = goAptProgressCallback;
    global_user_data = user_data;
}

// Wires the global log callback to the Go-side bridge function.
extern "C" void apt_enable_go_log_callback(const uintptr_t user_data) {
    g_log_callback = static_cast<AptLogCallback>(goAptLogCallback);
    g_log_user_data = user_data;
}
