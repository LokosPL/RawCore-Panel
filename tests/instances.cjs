const fs=require('fs'),http=require('http'),assert=require('assert');
const {chromium}=require('playwright');
const instances=[{id:'normalny',name:'Serwer główny',serverPath:'/srv/main',minecraftProcess:'main',wwwProcess:'www',port:25565,revision:'main-v1'},{id:'testowy',name:'Serwer testowy',serverPath:'/srv/test',minecraftProcess:'test',wwwProcess:'',port:25566,revision:'test-v1'}];
let pendingMain=null,requests=[],created=0,started=0;
const settings={host:'192.0.2.10',port:22,user:'minecraft',serverPath:'/srv/main',minecraftProcess:'main',wwwProcess:'www',autoUpdate:false,autoLogin:false,theme:'light',instances};
const server=http.createServer(async(req,res)=>{
 const url=new URL(req.url,'http://localhost');let body='';for await(const b of req)body+=b;
 if(url.pathname==='/'){res.setHeader('Content-Type','text/html');res.end(fs.readFileSync('assets/ui.html','utf8').replace('{{.CSRF}}','test'));return}
 if(url.pathname.startsWith('/assets/')){let p='.'+url.pathname;res.setHeader('Content-Type',p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':'image/png');res.end(fs.readFileSync(p));return}
 const id=req.headers['x-rawcraft-instance']||'normalny',instance=instances.find(i=>i.id===id);requests.push({url:url.pathname,id,body:body?JSON.parse(body):null});
 let out={};res.setHeader('Content-Type','application/json');
 switch(url.pathname){
  case '/api/settings':out={preferences:settings,hasToken:true,version:'1.4.8'};break;
  case '/api/instances':out={instances};break;
  case '/api/status':out={processes:[{pm_id:id==='normalny'?1:2,name:instance.minecraftProcess,monit:{cpu:1,memory:1},pm2_env:{status:'online'}}],metrics:{}};break;
  case '/api/files':if(id==='normalny'&&url.searchParams.get('path')==='/srv/main'){await new Promise(resolve=>pendingMain=resolve)}out={path:instance.serverPath,items:[{name:id+'.txt',dir:false,size:1,mtime:1,version:'v1'}]};break;
  case '/api/monitor':out={services:[],disks:[]};break;
  case '/api/payments/status':out={version:1,paymentsAvailable:false,observedAt:Date.now(),reason:'TEST_MODE'};break;
  case '/api/backups':out={folder:instance.serverPath+'/backup',items:[]};break;
  case '/api/minecraft/command':out={output:id+' reply'};break;
  case '/api/instances/catalog':if(JSON.parse(body).action==='create'){created++;const item=JSON.parse(body).instance;instances.push({...item,revision:'prepared-v1'})}out={planHash:'reviewed',prepared:JSON.parse(body).action==='create',started:false,javaVersion:'openjdk 25'};break;
  case '/api/instances/catalog-start':started++;out={started:true,pm2Saved:true};break;
 }
 res.end(JSON.stringify(out));
});
(async()=>{
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let browser=await chromium.launch({headless:true,args:['--no-sandbox']}),page=await browser.newPage({viewport:{width:1440,height:1100}}),errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto('http://127.0.0.1:'+server.address().port);await page.waitForFunction(()=>activeInstance?.id==='normalny');
 await page.evaluate(()=>enterApp({host:prefs.host,user:prefs.user}));
 await page.click('#nav-files');await page.waitForFunction(()=>document.querySelector('#filetbody').textContent.includes('Odczytywanie'));
 await page.selectOption('#instanceselect','testowy');await page.waitForFunction(()=>activeInstance?.id==='testowy'&&document.querySelector('#filetbody').textContent.includes('testowy.txt'));
 assert(pendingMain);pendingMain();await page.waitForTimeout(200);assert((await page.locator('#filetbody').innerText()).includes('testowy.txt'));assert(!(await page.locator('#filetbody').innerText()).includes('normalny.txt'));
 await page.click('#nav-console');await page.fill('#mccommand','core motd status');await page.click('#sendmc');await page.locator('#rconreply').filter({hasText:'testowy reply'}).waitFor();let rcon=requests.find(r=>r.url==='/api/minecraft/command');assert.equal(rcon.id,'testowy');assert.equal(rcon.body.path,'/srv/test');
 await page.click('#nav-backups');await page.locator('#backupfolder').filter({hasText:'/srv/test/backup'}).waitFor();
 await page.getByRole('button',{name:'+ Nowa instancja',exact:true}).click();await page.fill('#newinstanceid','lobby');await page.fill('#newinstancename','Lobby');await page.fill('#newinstancepath','/srv/lobby');await page.fill('#newinstanceprocess','lobby');await page.click('#instancepreview');await page.locator('#instancecreate').waitFor({state:'visible'});await page.click('#instancecreate');await page.waitForFunction(()=>document.querySelector('#instanceplan').textContent.includes('Paper i pluginy pobrano'));assert.equal(created,1);assert.equal(started,0);await page.locator('#instanceeulabox').waitFor({state:'visible'});
 await page.screenshot({path:'preview-multi-instance.png',fullPage:true});assert.deepEqual(errors,[]);await browser.close();server.close();console.log('PASS: instance switch clears views; delayed old file response ignored; RCON/backups bind to selected instance; reviewed wizard never starts PM2');
})().catch(e=>{console.error(e);process.exit(1)});
