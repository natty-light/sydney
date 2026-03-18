use std::ffi::{CStr, CString};
use std::net::TcpStream;
use std::os::raw::{c_char, c_int, c_long, c_void};
use std::os::unix::io::AsRawFd;
use std::sync::{LazyLock, Mutex};

use crate::error::LAST_ERROR;

// ---------------------------------------------------------------------------
// OpenSSL FFI declarations
// ---------------------------------------------------------------------------
extern "C" {
    fn OPENSSL_init_ssl(opts: u64, settings: *const c_void) -> c_int;

    fn TLS_client_method() -> *mut c_void;
    fn SSL_CTX_new(method: *mut c_void) -> *mut c_void;
    fn SSL_CTX_free(ctx: *mut c_void);
    fn SSL_CTX_set_default_verify_paths(ctx: *mut c_void) -> c_int;
    fn SSL_CTX_set_verify(ctx: *mut c_void, mode: c_int, callback: *const c_void);

    fn SSL_new(ctx: *mut c_void) -> *mut c_void;
    fn SSL_free(ssl: *mut c_void);
    fn SSL_set_fd(ssl: *mut c_void, fd: c_int) -> c_int;
    // SSL_set_tlsext_host_name is a macro — call SSL_ctrl directly
    fn SSL_ctrl(ssl: *mut c_void, cmd: c_int, larg: c_long, parg: *const c_char) -> c_long;
    fn SSL_connect(ssl: *mut c_void) -> c_int;
    fn SSL_read(ssl: *mut c_void, buf: *mut u8, num: c_int) -> c_int;
    fn SSL_write(ssl: *mut c_void, buf: *const u8, num: c_int) -> c_int;
    fn SSL_shutdown(ssl: *mut c_void) -> c_int;
    fn SSL_get_error(ssl: *mut c_void, ret: c_int) -> c_int;

    fn ERR_error_string_n(e: u64, buf: *mut c_char, len: usize);
    fn ERR_get_error() -> u64;
}

const SSL_VERIFY_PEER: c_int = 0x01;
const SSL_CTRL_SET_TLSEXT_HOSTNAME: c_int = 55;
const TLSEXT_NAMETYPE_host_name: c_long = 0;
const OPENSSL_INIT_FLAGS: u64 = 0x00200000 | 0x02000000;

// ---------------------------------------------------------------------------
// TLS stream storage — global Mutex like TCP
// ---------------------------------------------------------------------------
struct TlsStream {
    ssl: *mut c_void,
    _tcp: TcpStream, // kept alive so the fd stays valid
}

// SSL* pointers are not Send by default, but we only access them
// under the global Mutex, so one thread at a time.
unsafe impl Send for TlsStream {}

impl Drop for TlsStream {
    fn drop(&mut self) {
        unsafe {
            SSL_shutdown(self.ssl);
            SSL_free(self.ssl);
        }
    }
}

static TLS_STREAMS: LazyLock<Mutex<Vec<Option<TlsStream>>>> =
    LazyLock::new(|| Mutex::new(Vec::new()));

// Wrapper so *mut c_void can live in a global Mutex (implements Send).
struct Ptr(*mut c_void);
unsafe impl Send for Ptr {}

static SSL_CTX: LazyLock<Mutex<Option<Ptr>>> =
    LazyLock::new(|| Mutex::new(None));

fn slab_insert<T>(vec: &mut Vec<Option<T>>, val: T) -> usize {
    for (i, slot) in vec.iter_mut().enumerate() {
        if slot.is_none() {
            *slot = Some(val);
            return i;
        }
    }
    vec.push(Some(val));
    vec.len() - 1
}

fn set_error(msg: String) -> i64 {
    LAST_ERROR.with(|e| {
        *e.borrow_mut() = Some(CString::new(msg).unwrap());
    });
    -1
}

fn set_error_null(msg: String) -> *const c_char {
    LAST_ERROR.with(|e| {
        *e.borrow_mut() = Some(CString::new(msg).unwrap());
    });
    std::ptr::null()
}

fn get_openssl_error() -> String {
    unsafe {
        let code = ERR_get_error();
        if code == 0 {
            return "unknown SSL error".to_string();
        }
        let mut buf = [0u8; 256];
        ERR_error_string_n(code, buf.as_mut_ptr() as *mut c_char, buf.len());
        let msg = CStr::from_ptr(buf.as_ptr() as *const c_char);
        msg.to_string_lossy().into_owned()
    }
}

/// Lazily initialize the global SSL_CTX.
fn get_or_init_ctx() -> Result<*mut c_void, String> {
    let mut ctx_opt = SSL_CTX.lock().unwrap();
    if let Some(ref ctx) = *ctx_opt {
        return Ok(ctx.0);
    }

    unsafe {
        OPENSSL_init_ssl(OPENSSL_INIT_FLAGS, std::ptr::null());

        let method = TLS_client_method();
        if method.is_null() {
            return Err("tls: TLS_client_method returned null".to_string());
        }

        let ctx = SSL_CTX_new(method);
        if ctx.is_null() {
            return Err(format!("tls: SSL_CTX_new failed: {}", get_openssl_error()));
        }

        // Load the system's trusted CA certificates
        if SSL_CTX_set_default_verify_paths(ctx) != 1 {
            SSL_CTX_free(ctx);
            return Err("tls: failed to load system CA certificates".to_string());
        }

        // Require server certificate verification
        SSL_CTX_set_verify(ctx, SSL_VERIFY_PEER, std::ptr::null());

        *ctx_opt = Some(Ptr(ctx));
        Ok(ctx)
    }
}

// ---------------------------------------------------------------------------
// Public FFI functions
// ---------------------------------------------------------------------------

/// Connect to host:port over TLS.
/// Returns a TLS stream handle on success, -1 on error.
#[no_mangle]
pub extern "C" fn sydney_tls_connect(host: *const c_char, port: i64) -> i64 {
    if host.is_null() {
        return set_error("tls_connect: null host".to_string());
    }

    let host_str = unsafe { CStr::from_ptr(host) }
        .to_str()
        .unwrap_or("");

    let ctx = match get_or_init_ctx() {
        Ok(ctx) => ctx,
        Err(msg) => return set_error(msg),
    };

    // Open a plain TCP connection first
    let addr = format!("{}:{}", host_str, port);
    let tcp_stream = match TcpStream::connect(&addr) {
        Ok(s) => s,
        Err(err) => return set_error(format!("tls_connect: {}", err)),
    };

    let fd = tcp_stream.as_raw_fd();

    unsafe {
        let ssl = SSL_new(ctx);
        if ssl.is_null() {
            return set_error(format!("tls_connect: SSL_new failed: {}", get_openssl_error()));
        }

        if SSL_set_fd(ssl, fd) != 1 {
            SSL_free(ssl);
            return set_error("tls_connect: SSL_set_fd failed".to_string());
        }

        // Set SNI hostname — required by most modern servers
        let host_cstr = CString::new(host_str).unwrap();
        SSL_ctrl(ssl, SSL_CTRL_SET_TLSEXT_HOSTNAME, TLSEXT_NAMETYPE_host_name, host_cstr.as_ptr());

        // Perform the TLS handshake
        let ret = SSL_connect(ssl);
        if ret != 1 {
            let err_code = SSL_get_error(ssl, ret);
            let detail = get_openssl_error();
            SSL_free(ssl);
            return set_error(format!(
                "tls_connect: handshake failed (ssl_err={}, {})",
                err_code, detail
            ));
        }

        let stream = TlsStream { ssl, _tcp: tcp_stream };
        let mut streams = TLS_STREAMS.lock().unwrap();
        slab_insert(&mut streams, stream) as i64
    }
}

/// Read up to max_len bytes from a TLS stream.
/// Returns a pointer to a null-terminated C string, or null on error.
#[no_mangle]
pub extern "C" fn sydney_tls_read(handle: i64, max_len: i64) -> *const c_char {
    let mut streams = TLS_STREAMS.lock().unwrap();
    let idx = handle as usize;

    if idx >= streams.len() {
        return set_error_null("tls_read: invalid handle".to_string());
    }

    match &mut streams[idx] {
        None => set_error_null("tls_read: connection already closed".to_string()),
        Some(stream) => unsafe {
            let mut buf = vec![0u8; max_len as usize];
            let n = SSL_read(stream.ssl, buf.as_mut_ptr(), max_len as c_int);
            if n <= 0 {
                let err_code = SSL_get_error(stream.ssl, n);
                return set_error_null(format!(
                    "tls_read: SSL_read failed (ssl_err={})",
                    err_code
                ));
            }
            buf.truncate(n as usize);
            match CString::new(buf) {
                Ok(cs) => cs.into_raw(),
                Err(err) => set_error_null(format!("tls_read: {}", err)),
            }
        },
    }
}

/// Write data to a TLS stream.
/// Returns the number of bytes written on success, -1 on error.
#[no_mangle]
pub extern "C" fn sydney_tls_write(handle: i64, data: *const c_char, len: i64) -> i64 {
    if data.is_null() {
        return set_error("tls_write: null data".to_string());
    }

    let mut streams = TLS_STREAMS.lock().unwrap();
    let idx = handle as usize;

    if idx >= streams.len() {
        return set_error("tls_write: invalid handle".to_string());
    }

    match &mut streams[idx] {
        None => set_error("tls_write: connection already closed".to_string()),
        Some(stream) => unsafe {
            let n = SSL_write(stream.ssl, data as *const u8, len as c_int);
            if n <= 0 {
                let err_code = SSL_get_error(stream.ssl, n);
                return set_error(format!(
                    "tls_write: SSL_write failed (ssl_err={})",
                    err_code
                ));
            }
            n as i64
        },
    }
}

/// Close a TLS stream.
/// Returns 0 on success, -1 on error.
#[no_mangle]
pub extern "C" fn sydney_tls_close(handle: i64) -> i64 {
    let mut streams = TLS_STREAMS.lock().unwrap();
    let idx = handle as usize;

    if idx >= streams.len() {
        return set_error("tls_close: invalid handle".to_string());
    }

    if streams[idx].is_none() {
        return set_error("tls_close: already closed".to_string());
    }

    // Setting to None drops the TlsStream, which calls SSL_shutdown + SSL_free
    streams[idx] = None;
    0
}
