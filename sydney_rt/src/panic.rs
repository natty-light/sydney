use std::process;

#[no_mangle]
pub extern "C" fn sydney_panic(msg: *const i8) {
    let c_str = unsafe { std::ffi::CStr::from_ptr(msg) };
    let s = c_str.to_str().unwrap_or("unknown error");
    eprintln!("panic: {}", s);
    process::exit(1);
}