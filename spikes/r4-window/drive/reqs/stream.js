Promise.all([
  window.__spike.runStream("frame16","incr"),
  new Promise(r=>setTimeout(()=>window.__spike.pressKeys(60).then(r),500))
])
