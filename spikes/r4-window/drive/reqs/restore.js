import("/src/mygo.ts").then(async m=>{
  await m.SpikeService.winOp("toggleMaximize"); // restore from minimize
  await new Promise(r=>setTimeout(r,700));
  return m.SpikeService.winState()
})
