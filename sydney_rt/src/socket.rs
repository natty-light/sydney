use std::ffi::{CStr, CString};
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::raw::c_char;
use std::sync::{LazyLock, Mutex};

use crate::error::LAST_ERROR;

// Global storage for active connections.
// Each handle is an index into these vecs.
// Using Option so we can reuse slots after close.
// Mutex-protected so handles are valid across OS threads (spawn).
static STREAMS: LazyLock<Mutex<Vec<Option<TcpStream>>>> =
    LazyLock::new(|| Mutex::new(Vec::new()));

static LISTENERS: LazyLock<Mutex<Vec<Option<TcpListener>>>> =
    LazyLock::new(|| Mutex::new(Vec::new()));

/// Helper: insert a value into a slab vec, reusing empty slots.
/// Returns the index (handle).
fn slab_insert<T>(vec: &mut Vec<Option<T>>, val: T) -> usize {
    // Look for an empty slot to reuse
    for (i, slot) in vec.iter_mut().enumerate() {
        if slot.is_none() {
            *slot = Some(val);
            return i;
        }
    }
    // No empty slot, push to end
    vec.push(Some(val));
    vec.len() - 1
}

/// Helper: set LAST_ERROR and return -1.
fn set_error(msg: String) -> i64 {
    LAST_ERROR.with(|e| {
        *e.borrow_mut() = Some(CString::new(msg).unwrap());
    });
    -1
}


/// Connect to host:port over TCP.
/// Returns a stream handle (index) on success, -1 on error.
///
/// The host is a C string (e.g. "127.0.0.1" or "example.com").
/// Rust's TcpStream::connect handles DNS resolution automatically.
#[no_mangle]
pub extern "C" fn sydney_tcp_connect(host: *const c_char, port: i64) -> i64 {
    if host.is_null() {
        return set_error("tcp_connect: null host".to_string());
    }

    let host_str = unsafe { CStr::from_ptr(host) }
        .to_str()
        .unwrap_or("");

    // TcpStream::connect takes "host:port" and handles DNS + the TCP handshake.
    // This is a blocking call — it will wait until the connection is established
    // or an error occurs (connection refused, timeout, DNS failure, etc).
    let addr = format!("{}:{}", host_str, port);
    match TcpStream::connect(&addr) {
        Ok(stream) => {
            let mut streams = STREAMS.lock().unwrap();
            slab_insert(&mut streams, stream) as i64
        }
        Err(err) => set_error(format!("tcp_connect: {}", err)),
    }
}

/// Bind and listen on host:port.
/// Returns a listener handle on success, -1 on error.
///
/// Use "0.0.0.0" to listen on all interfaces, or "127.0.0.1" for localhost only.
/// The OS assigns a backlog queue automatically with Rust's TcpListener.
#[no_mangle]
pub extern "C" fn sydney_tcp_listen(host: *const c_char, port: i64) -> i64 {
    if host.is_null() {
        return set_error("tcp_listen: null host".to_string());
    }

    let host_str = unsafe { CStr::from_ptr(host) }
        .to_str()
        .unwrap_or("");

    // TcpListener::bind does three things under the hood:
    // 1. Creates a socket (socket syscall)
    // 2. Binds it to the address (bind syscall)
    // 3. Starts listening for connections (listen syscall)
    let addr = format!("{}:{}", host_str, port);
    match TcpListener::bind(&addr) {
        Ok(listener) => {
            let mut listeners = LISTENERS.lock().unwrap();
            slab_insert(&mut listeners, listener) as i64
        }
        Err(err) => set_error(format!("tcp_listen: {}", err)),
    }
}

/// Accept an incoming connection on a listener.
/// Blocks until a client connects.
/// Returns a new stream handle for the accepted connection, -1 on error.
///
/// Each accepted connection gets its own stream handle, independent of the
/// listener. The listener stays open and can accept more connections.
#[no_mangle]
pub extern "C" fn sydney_tcp_accept(listener_handle: i64) -> i64 {
    // Lock listeners to get the listener, then accept.
    // We need to be careful: accept() blocks, so we clone the listener
    // to avoid holding the lock during the blocking call.
    let listener_clone = {
        let listeners = LISTENERS.lock().unwrap();
        let idx = listener_handle as usize;

        if idx >= listeners.len() {
            return set_error("tcp_accept: invalid listener handle".to_string());
        }

        match &listeners[idx] {
            None => return set_error("tcp_accept: listener already closed".to_string()),
            Some(listener) => {
                // try_clone so we can drop the lock before blocking
                match listener.try_clone() {
                    Ok(l) => l,
                    Err(err) => return set_error(format!("tcp_accept: clone failed: {}", err)),
                }
            }
        }
    };
    // Lock is dropped here — accept can block without holding it
    match listener_clone.accept() {
        Ok((stream, _addr)) => {
            let mut streams = STREAMS.lock().unwrap();
            slab_insert(&mut streams, stream) as i64
        }
        Err(err) => set_error(format!("tcp_accept: {}", err)),
    }
}

/// Read up to max_len bytes from a stream.
/// Returns a pointer to a null-terminated C string containing the data read,
/// or null on error.
///
/// This is a blocking call — it waits until at least 1 byte is available
/// or the connection is closed (returns empty string).
///
/// Note: unlike the other functions that return i64, this returns a ptr
/// because we need to return the actual data. The caller (Sydney runtime
/// or emitter) is responsible for treating this as a Sydney string.
/// Returns null on error (check LAST_ERROR).
#[no_mangle]
pub extern "C" fn sydney_tcp_read(handle: i64, max_len: i64) -> *const c_char {
    let mut streams = STREAMS.lock().unwrap();
    let idx = handle as usize;

    if idx >= streams.len() {
        set_error("tcp_read: invalid handle".to_string());
        return std::ptr::null();
    }

    match &mut streams[idx] {
        None => {
            set_error("tcp_read: connection already closed".to_string());
            std::ptr::null()
        }
        Some(stream) => {
            // Allocate a buffer of max_len bytes.
            // read() fills as many bytes as are immediately available
            // (up to max_len), then returns. It does NOT wait for
            // max_len bytes — it returns as soon as there's data.
            // Returns 0 bytes when the remote end has closed the connection.
            let mut buf = vec![0u8; max_len as usize];
            match stream.read(&mut buf) {
                Ok(n) => {
                    buf.truncate(n);
                    // Convert to a C string. If the data contains null
                    // bytes, CString::new will error — for HTTP text
                    // this shouldn't happen, but handle it gracefully.
                    match CString::new(buf) {
                        Ok(cs) => cs.into_raw(),
                        Err(err) => {
                            set_error(format!("tcp_read: {}", err));
                            std::ptr::null()
                        }
                    }
                }
                Err(err) => {
                    set_error(format!("tcp_read: {}", err));
                    std::ptr::null()
                }
            }
        }
    }
}

/// Write data to a stream.
/// Returns the number of bytes written on success, -1 on error.
///
/// write_all is used instead of write to ensure all bytes are sent.
/// Plain write() might only send a partial buffer if the OS send buffer
/// is full — write_all loops internally until everything is sent.
#[no_mangle]
pub extern "C" fn sydney_tcp_write(handle: i64, data: *const c_char, len: i64) -> i64 {
    if data.is_null() {
        return set_error("tcp_write: null data".to_string());
    }

    let mut streams = STREAMS.lock().unwrap();
    let idx = handle as usize;

    if idx >= streams.len() {
        return set_error("tcp_write: invalid handle".to_string());
    }

    match &mut streams[idx] {
        None => set_error("tcp_write: connection already closed".to_string()),
        Some(stream) => {
            // Get the raw bytes from the C string pointer.
            // We use from_ptr + to_bytes to get the byte slice
            // without the null terminator.
            let bytes = unsafe {
                std::slice::from_raw_parts(data as *const u8, len as usize)
            };

            match stream.write_all(bytes) {
                Ok(_) => len,
                Err(err) => set_error(format!("tcp_write: {}", err)),
            }
        }
    }
}

/// Close a stream handle.
/// Returns 0 on success, -1 on error.
///
/// Sets the slab slot to None, which drops the TcpStream.
/// Dropping a TcpStream sends a FIN to the remote end (graceful close).
#[no_mangle]
pub extern "C" fn sydney_tcp_close_stream(handle: i64) -> i64 {
    let mut streams = STREAMS.lock().unwrap();
    let idx = handle as usize;

    if idx >= streams.len() {
        return set_error("tcp_close: invalid handle".to_string());
    }

    if streams[idx].is_none() {
        return set_error("tcp_close: already closed".to_string());
    }

    // Setting to None drops the TcpStream, which closes the socket.
    streams[idx] = None;
    0
}

/// Close a listener handle.
/// Returns 0 on success, -1 on error.
#[no_mangle]
pub extern "C" fn sydney_tcp_close_listener(listener_handle: i64) -> i64 {
    let mut listeners = LISTENERS.lock().unwrap();
    let idx = listener_handle as usize;

    if idx >= listeners.len() {
        return set_error("tcp_close_listener: invalid handle".to_string());
    }

    if listeners[idx].is_none() {
        return set_error("tcp_close_listener: already closed".to_string());
    }

    listeners[idx] = None;
    0
}
