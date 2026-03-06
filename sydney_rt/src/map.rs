use std::collections::HashMap;
use std::ffi::CStr;

#[no_mangle]
pub extern "C" fn sydney_map_create_int() -> *mut HashMap<i64, i64> {
  Box::into_raw(Box::new(HashMap::new()))
}
#[no_mangle]
pub extern "C" fn sydney_map_set_int(map: *mut HashMap<i64, i64>, key: i64, value: i64) {
  unsafe { (*map).insert(key, value); }
}
#[no_mangle]
pub extern "C" fn sydney_map_get_int(map: *const HashMap<i64, i64>, key: i64) -> i64 {
  unsafe { *(*map).get(&key).unwrap_or(&0) }
}
#[no_mangle]
pub extern "C" fn sydney_map_create_string() -> *mut HashMap<String, i64> {
  Box::into_raw(Box::new(HashMap::new()))
}
#[no_mangle]
pub extern "C" fn sydney_map_set_str(map: *mut HashMap<String, i64>, key: *const u8, value: i64) {
  unsafe {
    let k = CStr::from_ptr(key as *const i8).to_string_lossy().into_owned();
    (*map).insert(k, value);
  }
}

#[no_mangle]
pub extern "C" fn sydney_map_get_str(map: *const HashMap<String, i64>, key: *const u8) -> i64 {
  unsafe {
    let k = CStr::from_ptr(key as *const i8).to_string_lossy();
    *(*map).get(k.as_ref()).unwrap_or(&0)
  }
}