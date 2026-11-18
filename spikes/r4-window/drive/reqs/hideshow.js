import("/src/mygo.ts").then(async m=>{
  await m.SpikeService.winOp("hideToTray");
  await new Promise(r=>setTimeout(r,600));
  const h = await m.SpikeService.winState();
  await m.SpikeService.winOp("show");
  await new Promise(r=>setTimeout(r,600));
  const s = await m.SpikeService.winState();
  return {hidden:h.visible, shown:s.visible}
})
