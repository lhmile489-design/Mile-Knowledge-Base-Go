Promise.all([
  window.__spike.runStream("token","incr"),
  new Promise(r=>setTimeout(()=>window.__spike.pressKeys(60).then(r),500))
])
