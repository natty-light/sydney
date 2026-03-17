use std::sync::mpsc::{sync_channel, SyncSender, Receiver};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};

struct ChannelState {
    tx: SyncSender<i64>,
    rx: Mutex<Receiver<i64>>,
}

static CHANNELS: Mutex<Vec<Arc<ChannelState>>> = Mutex::new(Vec::new());
static THREADS: Mutex<Vec<JoinHandle<()>>> = Mutex::new(Vec::new());

#[no_mangle]
pub extern "C" fn sydney_channel_create(capacity: i64) -> i64 {
    let (tx, rx) = sync_channel(capacity as usize);
    let mut channels = CHANNELS.lock().unwrap();
    let id = channels.len() as i64;
    channels.push(Arc::new(ChannelState {
        tx,
        rx: Mutex::new(rx),
    }));
    id
}

#[no_mangle]
pub extern "C" fn sydney_channel_send(handle: i64, value: i64) {
    let tx = {
        let channels = CHANNELS.lock().unwrap();
        channels[handle as usize].tx.clone()
    };
    tx.send(value).unwrap();
}

#[no_mangle]
pub extern "C" fn sydney_channel_recv(handle: i64) -> i64 {
    let ch = {
        let channels = CHANNELS.lock().unwrap();
        Arc::clone(&channels[handle as usize])
    };
    let rx = ch.rx.lock().unwrap();
    rx.recv().unwrap()
}

#[no_mangle]
pub extern "C" fn sydney_spawn(fn_ptr: extern "C" fn(*mut u8), env_ptr: *mut u8) {
    let env = env_ptr as usize;
    let handle = thread::spawn(move || {
        let env = env as *mut u8;
        fn_ptr(env);
    });
    THREADS.lock().unwrap().push(handle);
}

#[no_mangle]
pub extern "C" fn sydney_join_all() {
    let mut threads = THREADS.lock().unwrap();
    for handle in threads.drain(..) {
        handle.join().unwrap();
    }
}
