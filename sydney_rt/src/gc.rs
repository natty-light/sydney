use std::alloc::{alloc, Layout};

#[no_mangle]
pub extern "C" fn sydney_gc_init() {
    // no-op for now
}

#[no_mangle]
pub extern "C" fn sydney_gc_alloc(size: i64) -> *mut u8 {
    if size <= 0 {
        return std::ptr::null_mut();
    }
    unsafe {
        let layout = Layout::from_size_align(size as usize, 8).unwrap();
        let ptr = alloc(layout);
        if ptr.is_null() {
            panic!("sydney_gc_alloc: out of memory");
        }
        ptr
    }
}

#[no_mangle]
pub extern "C" fn sydney_gc_collect() {
    // no-op for now
}

#[no_mangle]
pub extern "C" fn sydney_gc_add_global_root(_root: *const *mut u8) {
    // no-op for now
}

#[no_mangle]
pub extern "C" fn sydney_gc_shutdown() {
    // no-op for now
}