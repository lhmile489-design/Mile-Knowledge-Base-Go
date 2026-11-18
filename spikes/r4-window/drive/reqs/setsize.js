import("/src/mygo.ts").then(async m=>{
  await m.SpikeService.winOp("setSize");  // asks for 400x300, OS clamps to min
  await new Promise(r=>setTimeout(r,500));
  return m.SpikeService.winState()
})
