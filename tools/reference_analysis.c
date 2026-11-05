/* Copyright 2026 go-lc3 contributors
 * SPDX-License-Identifier: Apache-2.0
 * Oracle generator for liblc3 48bbd3eacd36e99a57317a0a4867002e0b09e183.
 * Compile with the same command as reference_spectrum.c, substituting this file.
 */
#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include "plc.h"
#include "attdet.h"
#include "energy.h"
#include "bwdet.h"
#include "tables.h"
static uint32_t hashfloats(float *x,int n){uint32_t h=2166136261u;for(int i=0;i<n;i++){uint32_t u;memcpy(&u,&x[i],4);for(int j=0;j<4;j++)h=(h^(uint8_t)(u>>(8*j)))*16777619u;}return h;}
int main(void){int first=1;puts("[");
 for(int dt=0;dt<4;dt++)for(int sr=0;sr<7;sr++)if(!(dt==2&&sr>=5)){
  struct lc3_attdet_analysis att={0};struct lc3_plc_state plc;lc3_plc_reset(&plc);
  for(int frame=0;frame<12;frame++){
   int ns=lc3_ns(dt,sr),nt=lc3_nt(sr),ne=lc3_ne(dt,sr),nb=lc3_num_bands[dt][sr];int16_t x[1080]={0};float spec[960]={0},e[64]={0},concealed[960]={0};uint32_t rng=0xdca12845u+(uint32_t)(dt*1000+sr*100+frame);
   for(int i=0;i<ns+nt;i++){rng=rng*1664525u+1013904223u;x[i]=(int16_t)(rng>>16);}
   for(int i=0;i<ne;i++){rng=rng*1664525u+1013904223u;spec[i]=((int)(rng>>16)-32768)*0.00390625f;if(frame%3==0&&i>ne*3/4)spec[i]*=0.0001f;if(frame%3==1&&i<ne*3/4)spec[i]*=0.0001f;}
   int nn=lc3_energy_compute(dt,sr,spec,e);int bw=lc3_bwdet_run(dt,sr,e);int attack=lc3_attdet_run(dt,sr,frame%3==0?20:frame%3==1?100:200,&att,x+nt);
   if(frame==2)lc3_plc_suspend(&plc);lc3_plc_synthesize(dt,sr,&plc,spec,concealed);
   uint32_t alpha;memcpy(&alpha,&plc.alpha,4);
   printf("%s{\"dt\":%d,\"sr\":%d,\"frame\":%d,\"energyHash\":%u,\"nn\":%d,\"bw\":%d,\"attack\":%d,\"en1\":%d,\"an1\":%d,\"patt\":%d,\"plcHash\":%u,\"seed\":%u,\"count\":%d,\"alpha\":%u}",first?"":",\n",dt,sr,frame,hashfloats(e,nb),nn,bw,attack,att.en1,att.an1,att.p_att,hashfloats(concealed,ne),plc.seed,plc.count,alpha);first=0;
  }
 }
 puts("\n]");
}
