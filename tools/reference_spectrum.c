/* Copyright 2026 go-lc3 contributors
 * SPDX-License-Identifier: Apache-2.0
 * Oracle generator for liblc3 48bbd3eacd36e99a57317a0a4867002e0b09e183.
 * cc -O2 -ffast-math -fno-associative-math -ffp-contract=off -U__ARM_NEON \
 *   -I "$LIBLC3/include" -I "$LIBLC3/src" tools/reference_spectrum.c \
 *   "$LIBLC3"/src/*.c -lm -o /tmp/reference_spectrum
 * /tmp/reference_spectrum > testdata/spectrum_reference.json
 */
#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include "spec.h"
#include "plc.h"
#include "attdet.h"
#include "energy.h"
#include "bwdet.h"
static uint32_t hashfloats(float *x,int n) {uint32_t h=2166136261u;for(int i=0;i<n;i++){uint32_t u;memcpy(&u,&x[i],4);for(int j=0;j<4;j++){h=(h^(uint8_t)(u>>(8*j)))*16777619u;}}return h;}
int main(void){
 puts("[");int first=1;
 for(int dt=0;dt<4;dt++)for(int sr=0;sr<7;sr++)if(!(dt==2&&sr>=5))for(int pattern=0;pattern<4;pattern++){
  int limits[3]={lc3_min_frame_bytes(dt,sr),0,lc3_max_frame_bytes(dt,sr)};limits[1]=(limits[0]+limits[2])/2;
  struct lc3_spec_analysis spec={0};
  for(int frame=0;frame<3;frame++){
   int nbytes=limits[frame],ne=lc3_ne(dt,sr);float x[960]={0},dec[960]={0};uint8_t out[625]={0};uint32_t rng=0x51a2f339u+(uint32_t)(dt*1000+sr*100+pattern*10+frame);
   for(int i=0;i<ne;i++){rng=rng*1664525u+1013904223u;x[i]=((int)(rng>>16)-32768)*0.03125f;if(pattern==0||((i+pattern)%13<7&&pattern==1))x[i]=0;if(pattern==2)x[i]*=0.0001f;if(pattern==3)x[i]*=128.f;}
   struct lc3_tns_data tns={0};struct lc3_spec_side side={0};int pitch=frame%2;
   lc3_spec_analyze(dt,sr,nbytes,pitch,&tns,&spec,x,&side);
   struct lc3_bits bits;lc3_setup_bits(&bits,LC3_BITS_MODE_WRITE,out,nbytes);lc3_spec_put_side(&bits,dt,sr,&side);lc3_spec_encode(&bits,dt,sr,sr,nbytes,&side,x);lc3_flush_bits(&bits);
   struct lc3_spec_side sd={0};lc3_setup_bits(&bits,LC3_BITS_MODE_READ,out,nbytes);int status=lc3_spec_get_side(&bits,dt,sr,&sd);if(!status)status=lc3_spec_decode(&bits,dt,sr,sr,nbytes,&sd,dec);if(!status)status=lc3_check_bits(&bits);
   uint32_t off;memcpy(&off,&spec.nbits_off,4);
   printf("%s{\"dt\":%d,\"sr\":%d,\"pattern\":%d,\"frame\":%d,\"nbytes\":%d,\"gain\":%d,\"nq\":%d,\"lsb\":%d,\"offset\":%u,\"spare\":%d,\"status\":%d,\"decodedHash\":%u,\"data\":\"",first?"":",\n",dt,sr,pattern,frame,nbytes,side.g_idx,side.nq,side.lsb_mode,off,spec.nbits_spare,status,hashfloats(dec,ne));
   for(int i=0;i<nbytes;i++)printf("%02x",out[i]);printf("\"}");first=0;
  }
 }
 puts("\n]");
}
