import("/src/mygo.ts").then(async m=>{
  await m.SpikeService.winOp("minimize");
  await new Promise(r=>setTimeout(r,900));
  const s1 = await m.SpikeService.winState();
  await m.SpikeService.winOp("toggleMaximize");
  await new Promise(r=>setTimeout(r,700));
  return {afterMin:s1, note:"restored via toggleMaximize"};
})
