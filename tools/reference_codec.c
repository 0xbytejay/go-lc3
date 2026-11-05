/* Copyright 2026 go-lc3 contributors
 * SPDX-License-Identifier: Apache-2.0
 * Whole-codec oracle generator. See generate_codec_reference.py.
 */
#include <stdio.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include "lc3.h"
#include "tables.h"
static void put32(uint32_t x){for(int i=0;i<4;i++)putchar((uint8_t)(x>>(8*i)));}
static void putblock(const void *p,uint32_t n){put32(n);fwrite(p,1,n,stdout);}
static void putword(uint8_t *p,uint32_t x,int n){for(int i=0;i<n;i++)p[i]=(uint8_t)(x>>(8*i));}
static const int sine16[16]={0,12539,23170,30273,32767,30273,23170,12539,0,-12539,-23170,-30273,-32767,-30273,-23170,-12539};
static void fillpcm(uint8_t *out,int dt,int sr,int pcm,int frame,int fmt,int stride){
 int n=lc3_ns(dt,pcm),width=(int[]){2,4,3,4}[fmt];uint32_t rng=0x6a7f31e9u+(uint32_t)(frame*31+dt*1000+sr*100+pcm*10000);
 memset(out,0xa5,n*stride*width);
 for(int i=0;i<n;i++){
  int t=frame*n+i;int32_t sample=0;rng=rng*1664525u+1013904223u;
  switch(frame/4){
   case 0:break;
   case 1:sample=sine16[((t*7+(sr+1)*11)&255)>>4]*192;break;
   case 2:{int v=sine16[((t*3)&255)>>4]+sine16[((t*7)&255)>>4]/2;int env=4+(t/48)%12;sample=v*env*8;break;}
   case 3:sample=((int)(rng>>16)-32768)*192;break;
   case 4:sample=sine16[((t*11)&255)>>4]*255;break;
   case 5:sample=((int)(rng>>16)-32768)/64;break;
   case 6:sample=t%127==0?8388607:t%127==1?-8388608:0;break;
   default:break;
  }
  uint8_t *p=out+i*stride*width;
  if(fmt==LC3_PCM_FORMAT_S16)putword(p,(uint32_t)(sample>>8),2);
  else if(fmt==LC3_PCM_FORMAT_FLOAT){float v=(float)sample*0x1p-23f;uint32_t u;memcpy(&u,&v,4);putword(p,u,4);}
  else putword(p,(uint32_t)sample,width);
 }
}
int main(void){
 const int hz[]={8000,16000,24000,32000,48000,48000,96000};
 if(*(const unsigned char *)&(const uint32_t){1}!=1){fprintf(stderr,"little-endian oracle host required\n");return 1;}
 fwrite("GOLC3REF1",1,9,stdout);put32(276);put32(32);
 for(int dt=0;dt<4;dt++)for(int sr=0;sr<7;sr++)if(!(dt==2&&sr>=5))for(int pcm=sr;pcm<(sr<5?5:7);pcm++)for(int fmt=0;fmt<4;fmt++){
  int hr=sr>=5,us=2500*(1+dt),ns=lc3_ns(dt,pcm),lo=lc3_min_frame_bytes(dt,sr),hi=lc3_max_frame_bytes(dt,sr);
  void *emem=calloc(1,lc3_hr_encoder_size(hr,us,hz[pcm]));void *dmem=calloc(1,lc3_hr_decoder_size(hr,us,hz[pcm]));
  lc3_encoder_t enc=lc3_hr_setup_encoder(hr,us,hz[sr],hz[pcm],emem);lc3_decoder_t dec=lc3_hr_setup_decoder(hr,us,hz[sr],hz[pcm],dmem);
  if(!enc||!dec)return 2;put32(dt);put32(sr);put32(pcm);put32(fmt);
  for(int frame=0;frame<32;frame++){
   int sizes[]={lo,(lo+hi)/2,hi,lo+5<hi?lo+5:hi};int nbytes=sizes[frame%4],stride=frame%3+1,outfmt=(frame+fmt)%4,width=(int[]){2,4,3,4}[outfmt];
   union {uint32_t align;uint8_t data[960*4*3];} input,output;uint8_t data[625]={0},corrupt[625];
   fillpcm(input.data,dt,sr,pcm,frame,fmt,stride);memset(output.data,0xa5,sizeof(output.data));
   if(lc3_encode(enc,fmt,input.data,stride,nbytes,data))return 3;
   const uint8_t *in=data;int inbytes=nbytes;
   if((frame>=8&&frame<=16)||frame==23){in=NULL;inbytes=0;}
   if(frame==20){memset(corrupt,0xff,nbytes);in=corrupt;}
   int status=lc3_decode(dec,in,inbytes,outfmt,output.data,stride);
   put32(outfmt);put32(stride);put32((uint32_t)status);putblock(data,nbytes);putblock(output.data,ns*stride*width);
  }
  free(emem);free(dmem);
 }
 return ferror(stdout)?4:0;
}
