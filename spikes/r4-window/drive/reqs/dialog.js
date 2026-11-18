import("/src/mygo.ts").then(async m=>{
  const p = m.SpikeService.dialogProbe(); // modal: blocks until user answers
  // give the automation a beat, then it just stays open for manual click
  return p
})
