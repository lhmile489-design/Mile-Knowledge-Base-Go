import("/src/mygo.ts").then(async m=>{
  await m.SpikeService.winOp("hideToTray");
  await new Promise(r=>setTimeout(r,600));
  return m.SpikeService.winState()
})
