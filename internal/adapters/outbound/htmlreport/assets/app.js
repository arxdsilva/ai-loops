(function(){
  var sel=document.getElementById('lane-filter');
  if(!sel)return;
  sel.addEventListener('change',function(){
    var v=sel.value;
    var rows=document.querySelectorAll('#steps tbody tr');
    for(var i=0;i<rows.length;i++){
      rows[i].hidden=(v!==''&&rows[i].getAttribute('data-lane')!==v);
    }
  });
})();
