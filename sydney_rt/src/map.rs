#[no_mangle]
pub extern "C" fn sydney_map_create() -> *mut HashMap<i64, i64> {
  Box::into_raw(Box::new(HashMap::new()))
}

#[no_mangle]
pub extern "C" fn sydney_map_set(map: *mut HashMap<i64, i64>, key: i64, value: i64) {
  unsafe { (*map).insert(key, value); }
}

#[no_mangle]
pub extern "C" fn sydney_map_get(map: *const HashMap<i64, i64>, key: i64) -> i64 {
  unsafe { *(*map).get(&key).unwrap_or(&0) }
}