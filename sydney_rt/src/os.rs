use std::ffi::{CStr, CString};

use libc::c_char;

use crate::gc::sydney_gc_alloc;

#[no_mangle]
pub extern "C" fn sydney_get_args() -> *mut u8 {
    let args: Vec<String> = std::env::args().collect();
    let len = args.len() as i64;
    let buf = sydney_gc_alloc(len * 8);
    unsafe {
        let data = buf as *mut *mut c_char;
        for (i, v) in args.iter().enumerate() {
            let cstr = CString::new(v.as_str()).unwrap();
            *data.add(i) = cstr.into_raw();
        }
        let header = sydney_gc_alloc(16) as *mut i64;
        *header = len;
        *header.add(1) = buf as i64;
        header as *mut u8
    }
}
