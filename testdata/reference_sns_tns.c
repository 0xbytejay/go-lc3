/*
 * Copyright 2026 The go-lc3 Authors
 * SPDX-License-Identifier: Apache-2.0
 *
 * Deterministic SNS/TNS fixture generator against Google liblc3 1.1.2,
 * revision 48bbd3eacd36e99a57317a0a4867002e0b09e183.
 *
 * The generated fixture contains only synthetic input cases. Build from
 * the repository root after setting LC3_SRC to an upstream liblc3 checkout:
 *
 * clang -O2 -ffast-math -fno-associative-math -ffp-contract=off \
 *   -U__ARM_NEON -I"$LC3_SRC/include" -I"$LC3_SRC/src" \
 *   testdata/reference_sns_tns.c "$LC3_SRC"/src/{sns,tns,bits,tables}.c \
 *   -o /tmp/lc3-sns-tns-reference
 * /tmp/lc3-sns-tns-reference > testdata/sns_tns_scalar.json
 *
 * Normal Go tests use the committed JSON and do not require a C compiler.
 */
#include <stdio.h>
#include <stdint.h>
#include <string.h>
#include "sns.h"
#include "tns.h"
#include "tables.h"
static uint32_t rng;
static uint32_t next(void) { rng^=rng<<13;rng^=rng>>17;rng^=rng<<5;return rng; }
static uint32_t bytes_crc(const uint8_t *x,int n) {
    uint32_t c=~0u;
    for(int i=0;i<n;i++) { c^=x[i];for(int k=0;k<8;k++)c=(c>>1)^((0u-(c&1))&0xedb88320u); }
    return ~c;
}
static uint32_t crc(const float *x,int n) {
    uint32_t c=~0u;
    for(int i=0;i<n;i++) {
        uint32_t v;memcpy(&v,&x[i],4);
        for(int b=0;b<4;b++) { c^=v&255;v>>=8;for(int k=0;k<8;k++)c=(c>>1)^((0u-(c&1))&0xedb88320u); }
    }
    return ~c;
}
int main(void) {
    puts("["); int count=0;
    for(int dt=0;dt<4;dt++)for(int sr=0;sr<7;sr++) {
        if(dt==2 && sr>=5) continue;
        for(int pattern=0;pattern<3;pattern++) {
            int nb=lc3_num_bands[dt][sr],ne=lc3_ne(dt,sr);
            int nbytes=pattern==0?lc3_min_frame_bytes(dt,sr):pattern==1?(lc3_min_frame_bytes(dt,sr)+lc3_max_frame_bytes(dt,sr))/2:lc3_max_frame_bytes(dt,sr);
            int att=pattern==1;
            rng=0x14912be1u+count;
            float eb[64],x[960],y[960],z[960],t[960],u[960];
            for(int i=0;i<nb;i++)eb[i]=((next()&65535)+1)/16.f;
            int walk=10000;
            for(int i=0;i<ne;i++) {
                int r=(int)(next()&65535)-32768;
                if(pattern==1) { walk+=(r%257);r=walk; }
                if(pattern==2) r=(i%128)*128-8192;
                x[i]=r/16.f;
            }
            struct lc3_sns_data sns={0};
            lc3_sns_analyze(dt,sr,nbytes,eb,att,&sns,x,y);
            lc3_sns_synthesize(dt,sr,&sns,x,z);
            struct lc3_tns_data tns={0};
            memcpy(t,x,sizeof(float)*ne);memcpy(u,x,sizeof(float)*ne);
            lc3_tns_analyze(dt,sr,false,nbytes,&tns,t);
            lc3_tns_synthesize(dt,sr,&tns,u);
            uint8_t encoded[625]={0};lc3_bits_t bits;
            lc3_setup_bits(&bits,LC3_BITS_MODE_WRITE,encoded,nbytes);
            lc3_tns_put_data(&bits,&tns);lc3_sns_put_data(&bits,&sns);lc3_flush_bits(&bits);
            printf("%s{\"dt\":%d,\"sr\":%d,\"pattern\":%d,\"nbytes\":%d,\"att\":%s,\"sns\":[%d,%d,%d,%d,%d,%d,%d,%d],\"sns_forward\":%u,\"sns_inverse\":%u,\"tns_weight\":%s,\"tns_nfilters\":%d,\"tns_order\":[%d,%d],\"tns_rc\":[",
              count?",\n":"",dt,sr,pattern,nbytes,att?"true":"false",sns.lfcb,sns.hfcb,sns.shape,sns.gain,sns.idx_a,sns.idx_b,sns.ls_a,sns.ls_b,
              crc(y,ne),crc(z,ne),tns.lpc_weighting?"true":"false",tns.nfilters,tns.rc_order[0],tns.rc_order[1]);
            for(int f=0;f<2;f++){printf("%s[",f?",":"");for(int i=0;i<8;i++)printf("%s%d",i?",":"",tns.rc[f][i]);printf("]");}
            printf("],\"tns_forward\":%u,\"tns_inverse\":%u,\"tns_nbits\":%d,\"bitstream\":%u}",crc(t,ne),crc(u,ne),lc3_tns_get_nbits(&tns),bytes_crc(encoded,nbytes));
            count++;
        }
    }
    puts("\n]");
}
