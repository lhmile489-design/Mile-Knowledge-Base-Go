import("/src/mygo.ts").then(async m=>{const t0=Date.now();const r=await m.SpikeService.winOp("close");return {r,ms:Date.now()-t0}})
