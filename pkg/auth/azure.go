/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	xpath "github.com/freiheit-com/kuberpult/pkg/path"

	"github.com/MicahParks/keyfunc/v2"
	jwt "github.com/golang-jwt/jwt/v5"
)

func JWKSInitAzureFromJson() (*keyfunc.JWKS, error) {
	// this is the result of auth.JWKSInitAzure:
	var staticJsonJwksString = "{\"keys\":[{\"kty\":\"RSA\",\"use\":\"sig\",\"kid\":\"nOo3ZDrODXEK1jKWhXslHR_KXEg\",\"x5t\":\"nOo3ZDrODXEK1jKWhXslHR_KXEg\",\"n\":\"oaLLT9hkcSj2tGfZsjbu7Xz1Krs0qEicXPmEsJKOBQHauZ_kRM1HdEkgOJbUznUspE6xOuOSXjlzErqBxXAu4SCvcvVOCYG2v9G3-uIrLF5dstD0sYHBo1VomtKxzF90Vslrkn6rNQgUGIWgvuQTxm1uRklYFPEcTIRw0LnYknzJ06GC9ljKR617wABVrZNkBuDgQKj37qcyxoaxIGdxEcmVFZXJyrxDgdXh9owRmZn6LIJlGjZ9m59emfuwnBnsIQG7DirJwe9SXrLXnexRQWqyzCdkYaOqkpKrsjuxUj2-MHX31FqsdpJJsOAvYXGOYBKJRjhGrGdONVrZdUdTBQ\",\"e\":\"AQAB\",\"x5c\":[\"MIIDBTCCAe2gAwIBAgIQN33ROaIJ6bJBWDCxtmJEbjANBgkqhkiG9w0BAQsFADAtMSswKQYDVQQDEyJhY2NvdW50cy5hY2Nlc3Njb250cm9sLndpbmRvd3MubmV0MB4XDTIwMTIyMTIwNTAxN1oXDTI1MTIyMDIwNTAxN1owLTErMCkGA1UEAxMiYWNjb3VudHMuYWNjZXNzY29udHJvbC53aW5kb3dzLm5ldDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAKGiy0/YZHEo9rRn2bI27u189Sq7NKhInFz5hLCSjgUB2rmf5ETNR3RJIDiW1M51LKROsTrjkl45cxK6gcVwLuEgr3L1TgmBtr/Rt/riKyxeXbLQ9LGBwaNVaJrSscxfdFbJa5J+qzUIFBiFoL7kE8ZtbkZJWBTxHEyEcNC52JJ8ydOhgvZYykete8AAVa2TZAbg4ECo9+6nMsaGsSBncRHJlRWVycq8Q4HV4faMEZmZ+iyCZRo2fZufXpn7sJwZ7CEBuw4qycHvUl6y153sUUFqsswnZGGjqpKSq7I7sVI9vjB199RarHaSSbDgL2FxjmASiUY4RqxnTjVa2XVHUwUCAwEAAaMhMB8wHQYDVR0OBBYEFI5mN5ftHloEDVNoIa8sQs7kJAeTMA0GCSqGSIb3DQEBCwUAA4IBAQBnaGnojxNgnV4+TCPZ9br4ox1nRn9tzY8b5pwKTW2McJTe0yEvrHyaItK8KbmeKJOBvASf+QwHkp+F2BAXzRiTl4Z+gNFQULPzsQWpmKlz6fIWhc7ksgpTkMK6AaTbwWYTfmpKnQw/KJm/6rboLDWYyKFpQcStu67RZ+aRvQz68Ev2ga5JsXlcOJ3gP/lE5WC1S0rjfabzdMOGP8qZQhXk4wBOgtFBaisDnbjV5pcIrjRPlhoCxvKgC/290nZ9/DLBH3TbHk8xwHXeBAnAjyAqOZij92uksAv7ZLq4MODcnQshVINXwsYshG1pQqOLwMertNaY5WtrubMRku44Dw7R\"],\"issuer\":\"https://login.microsoftonline.com/{tenantid}/v2.0\"},{\"kty\":\"RSA\",\"use\":\"sig\",\"kid\":\"l3sQ-50cCH4xBVZLHTGwnSR7680\",\"x5t\":\"l3sQ-50cCH4xBVZLHTGwnSR7680\",\"n\":\"sfsXMXWuO-dniLaIELa3Pyqz9Y_rWff_AVrCAnFSdPHa8__Pmkbt_yq-6Z3u1o4gjRpKWnrjxIh8zDn1Z1RS26nkKcNg5xfWxR2K8CPbSbY8gMrp_4pZn7tgrEmoLMkwfgYaVC-4MiFEo1P2gd9mCdgIICaNeYkG1bIPTnaqquTM5KfT971MpuOVOdM1ysiejdcNDvEb7v284PYZkw2imwqiBY3FR0sVG7jgKUotFvhd7TR5WsA20GS_6ZIkUUlLUbG_rXWGl0YjZLS_Uf4q8Hbo7u-7MaFn8B69F6YaFdDlXm_A0SpedVFWQFGzMsp43_6vEzjfrFDJVAYkwb6xUQ\",\"e\":\"AQAB\",\"x5c\":[\"MIIDBTCCAe2gAwIBAgIQWPB1ofOpA7FFlOBk5iPaNTANBgkqhkiG9w0BAQsFADAtMSswKQYDVQQDEyJhY2NvdW50cy5hY2Nlc3Njb250cm9sLndpbmRvd3MubmV0MB4XDTIxMDIwNzE3MDAzOVoXDTI2MDIwNjE3MDAzOVowLTErMCkGA1UEAxMiYWNjb3VudHMuYWNjZXNzY29udHJvbC53aW5kb3dzLm5ldDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALH7FzF1rjvnZ4i2iBC2tz8qs/WP61n3/wFawgJxUnTx2vP/z5pG7f8qvumd7taOII0aSlp648SIfMw59WdUUtup5CnDYOcX1sUdivAj20m2PIDK6f+KWZ+7YKxJqCzJMH4GGlQvuDIhRKNT9oHfZgnYCCAmjXmJBtWyD052qqrkzOSn0/e9TKbjlTnTNcrIno3XDQ7xG+79vOD2GZMNopsKogWNxUdLFRu44ClKLRb4Xe00eVrANtBkv+mSJFFJS1Gxv611hpdGI2S0v1H+KvB26O7vuzGhZ/AevRemGhXQ5V5vwNEqXnVRVkBRszLKeN/+rxM436xQyVQGJMG+sVECAwEAAaMhMB8wHQYDVR0OBBYEFLlRBSxxgmNPObCFrl+hSsbcvRkcMA0GCSqGSIb3DQEBCwUAA4IBAQB+UQFTNs6BUY3AIGkS2ZRuZgJsNEr/ZEM4aCs2domd2Oqj7+5iWsnPh5CugFnI4nd+ZLgKVHSD6acQ27we+eNY6gxfpQCY1fiN/uKOOsA0If8IbPdBEhtPerRgPJFXLHaYVqD8UYDo5KNCcoB4Kh8nvCWRGPUUHPRqp7AnAcVrcbiXA/bmMCnFWuNNahcaAKiJTxYlKDaDIiPN35yECYbDj0PBWJUxobrvj5I275jbikkp8QSLYnSU/v7dMDUbxSLfZ7zsTuaF2Qx+L62PsYTwLzIFX3M8EMSQ6h68TupFTi5n0M2yIXQgoRoNEDWNJZ/aZMY/gqT02GQGBWrh+/vJ\"],\"issuer\":\"https://login.microsoftonline.com/{tenantid}/v2.0\"},{\"kty\":\"RSA\",\"use\":\"sig\",\"kid\":\"Mr5-AUibfBii7Nd1jBebaxboXW0\",\"x5t\":\"Mr5-AUibfBii7Nd1jBebaxboXW0\",\"n\":\"yr3v1uETrFfT17zvOiy01w8nO-1t67cmiZLZxq2ISDdte9dw-IxCR7lPV2wezczIRgcWmYgFnsk2j6m10H4tKzcqZM0JJ_NigY29pFimxlL7_qXMB1PorFJdlAKvp5SgjSTwLrXjkr1AqWwbpzG2yZUNN3GE8GvmTeo4yweQbNCd-yO_Zpozx0J34wHBEMuaw-ZfCUk7mdKKsg-EcE4Zv0Xgl9wP2MpKPx0V8gLazxe6UQ9ShzNuruSOncpLYJN_oQ4aKf5ptOp1rsfDY2IK9frtmRTKOdQ-MEmSdjGL_88IQcvCs7jqVz53XKoXRlXB8tMIGOcg-ICer6yxe2itIQ\",\"e\":\"AQAB\",\"x5c\":[\"MIIDBTCCAe2gAwIBAgIQff8yrFO3CINPHUTT76tUsTANBgkqhkiG9w0BAQsFADAtMSswKQYDVQQDEyJhY2NvdW50cy5hY2Nlc3Njb250cm9sLndpbmRvd3MubmV0MB4XDTIxMTAyNDE3NDU1NloXDTI2MTAyNDE3NDU1NlowLTErMCkGA1UEAxMiYWNjb3VudHMuYWNjZXNzY29udHJvbC53aW5kb3dzLm5ldDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMq979bhE6xX09e87zostNcPJzvtbeu3JomS2catiEg3bXvXcPiMQke5T1dsHs3MyEYHFpmIBZ7JNo+ptdB+LSs3KmTNCSfzYoGNvaRYpsZS+/6lzAdT6KxSXZQCr6eUoI0k8C6145K9QKlsG6cxtsmVDTdxhPBr5k3qOMsHkGzQnfsjv2aaM8dCd+MBwRDLmsPmXwlJO5nSirIPhHBOGb9F4JfcD9jKSj8dFfIC2s8XulEPUoczbq7kjp3KS2CTf6EOGin+abTqda7Hw2NiCvX67ZkUyjnUPjBJknYxi//PCEHLwrO46lc+d1yqF0ZVwfLTCBjnIPiAnq+ssXtorSECAwEAAaMhMB8wHQYDVR0OBBYEFDiZG6s5d9RvorpqbVdS2/MD8ZKhMA0GCSqGSIb3DQEBCwUAA4IBAQAQAPuqqKj2AgfC9ayx+qUu0vjzKYdZ6T+3ssJDOGwB1cLMXMTUVgFwj8bsX1ahDUJdzKpWtNj7bno+Ug85IyU7k89U0Ygr55zWU5h4wnnRrCu9QKvudUPnbiXoVuHPwcK8w1fdXZQB5Qq/kKzhNGY57cG1bwj3R/aIdCp+BjgFppOKjJpK7FKS8G2v70eIiCLMapK9lLEeQOxIvzctTsXy9EZ7wtaIiYky4ZSituphToJUkakHaQ6evbn82lTg6WZz1tmSmYnPqRdAff7aiQ1Sw9HpuzlZY/piTVqvd6AfKZqyxu/FhENE0Odv/0hlHzI15jKQWL1Ljc0Nm3y1skut\"],\"issuer\":\"https://login.microsoftonline.com/{tenantid}/v2.0\"},{\"kty\":\"RSA\",\"use\":\"sig\",\"kid\":\"jS1Xo1OWDj_52vbwGNgvQO2VzMc\",\"x5t\":\"jS1Xo1OWDj_52vbwGNgvQO2VzMc\",\"n\":\"spvQcXWqYrMcvcqQmfSMYnbUC8U03YctnXyLIBe148OzhBrgdAOmPfMfJi_tUW8L9svVGpk5qG6dN0n669cRHKqU52GnG0tlyYXmzFC1hzHVgQz9ehve4tlJ7uw936XIUOAOxx3X20zdpx7gm4zHx4j2ZBlXskAj6U3adpHQNuwUE6kmngJWR-deWlEigMpRsvUVQ2O5h0-RSq8Wr_x7ud3K6GTtrzARamz9uk2IXatKYdnj5Jrk2jLY6nWt-GtxlA_l9XwIrOl6Sqa_pOGIpS01JKdxKvpBC9VdS8oXB-7P5qLksmv7tq-SbbiOec0cvU7WP7vURv104V4FiI_qoQ\",\"e\":\"AQAB\",\"x5c\":[\"MIIDBTCCAe2gAwIBAgIQHsetP+i8i6VIAmjmfVGv6jANBgkqhkiG9w0BAQsFADAtMSswKQYDVQQDEyJhY2NvdW50cy5hY2Nlc3Njb250cm9sLndpbmRvd3MubmV0MB4XDTIyMDEzMDIzMDYxNFoXDTI3MDEzMDIzMDYxNFowLTErMCkGA1UEAxMiYWNjb3VudHMuYWNjZXNzY29udHJvbC53aW5kb3dzLm5ldDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALKb0HF1qmKzHL3KkJn0jGJ21AvFNN2HLZ18iyAXtePDs4Qa4HQDpj3zHyYv7VFvC/bL1RqZOahunTdJ+uvXERyqlOdhpxtLZcmF5sxQtYcx1YEM/Xob3uLZSe7sPd+lyFDgDscd19tM3ace4JuMx8eI9mQZV7JAI+lN2naR0DbsFBOpJp4CVkfnXlpRIoDKUbL1FUNjuYdPkUqvFq/8e7ndyuhk7a8wEWps/bpNiF2rSmHZ4+Sa5Noy2Op1rfhrcZQP5fV8CKzpekqmv6ThiKUtNSSncSr6QQvVXUvKFwfuz+ai5LJr+7avkm24jnnNHL1O1j+71Eb9dOFeBYiP6qECAwEAAaMhMB8wHQYDVR0OBBYEFGzVFjAbYpU/2en4ry4LMLUHJ3GjMA0GCSqGSIb3DQEBCwUAA4IBAQBU0YdNVfdByvpwsPfwNdD8m1PLeeCKmLHQnWRI5600yEHuoUvoAJd5dwe1ZU1bHHRRKWN7AktUzofP3yF61xtizhEbyPjHK1tnR+iPEviWxVvK37HtfEPzuh1Vqp08bqY15McYUtf77l2HXTpak+UWYRYJBi++2umIDKY5UMqU+LEZnvaXybLUKN3xG4iy2q1Ab8syGFaUP7J3nCtVrR7ip39BnvSTTZZNo/OC7fYXJ2X4sN1/2ZhR5EtnAgwi2RvlZl0aWPrczArUCxDBCbsKPL/Up/kID1ir1VO4LT09ryfv2nx3y6l0YvuL7ePz4nGYCWHcbMVcUrQUXquZ3XtI\"],\"issuer\":\"https://login.microsoftonline.com/{tenantid}/v2.0\"},{\"kty\":\"RSA\",\"use\":\"sig\",\"kid\":\"2ZQpJ3UpbjAYXYGaXEJl8lV0TOI\",\"x5t\":\"2ZQpJ3UpbjAYXYGaXEJl8lV0TOI\",\"n\":\"wEMMJtj9yMQd8QS6Vnm538K5GN1Pr_I31_LUl9-OCYu-9_DrDvPGjViQK9kOiCjBfyqoAL-pBecn9-XXaS-C4xZTn1ZRw--GELabuo0u-U6r3TKj42xFDEP-_R5RpOGshoC95lrKiU5teuhn4fBM3XfR2GB0dVMcpzN3h4-0OMvBK__Zr9tkQCU_KzXTbNCjyA7ybtbr83NF9k3KjpTyOyY2S-qvFbY-AoqMhL9Rp8r2HBj_vrsr6RX6GeiSxxjbEzDFA2VIcSKbSHvbNBEeW2KjLXkz6QG2LjKz5XsYLp6kv_-k9lPQBy_V7Ci4ZkhAN-6j1S1Kcq58aLbp0wDNKQ\",\"e\":\"AQAB\",\"x5c\":[\"MIIDBTCCAe2gAwIBAgIQH4FlYNA+UJlF0G3vy9ZrhTANBgkqhkiG9w0BAQsFADAtMSswKQYDVQQDEyJhY2NvdW50cy5hY2Nlc3Njb250cm9sLndpbmRvd3MubmV0MB4XDTIyMDUyMjIwMDI0OVoXDTI3MDUyMjIwMDI0OVowLTErMCkGA1UEAxMiYWNjb3VudHMuYWNjZXNzY29udHJvbC53aW5kb3dzLm5ldDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMBDDCbY/cjEHfEEulZ5ud/CuRjdT6/yN9fy1JffjgmLvvfw6w7zxo1YkCvZDogowX8qqAC/qQXnJ/fl12kvguMWU59WUcPvhhC2m7qNLvlOq90yo+NsRQxD/v0eUaThrIaAveZayolObXroZ+HwTN130dhgdHVTHKczd4ePtDjLwSv/2a/bZEAlPys102zQo8gO8m7W6/NzRfZNyo6U8jsmNkvqrxW2PgKKjIS/UafK9hwY/767K+kV+hnokscY2xMwxQNlSHEim0h72zQRHltioy15M+kBti4ys+V7GC6epL//pPZT0Acv1ewouGZIQDfuo9UtSnKufGi26dMAzSkCAwEAAaMhMB8wHQYDVR0OBBYEFLFr+sjUQ+IdzGh3eaDkzue2qkTZMA0GCSqGSIb3DQEBCwUAA4IBAQCiVN2A6ErzBinGYafC7vFv5u1QD6nbvY32A8KycJwKWy1sa83CbLFbFi92SGkKyPZqMzVyQcF5aaRZpkPGqjhzM+iEfsR2RIf+/noZBlR/esINfBhk4oBruj7SY+kPjYzV03NeY0cfO4JEf6kXpCqRCgp9VDRM44GD8mUV/ooN+XZVFIWs5Gai8FGZX9H8ZSgkIKbxMbVOhisMqNhhp5U3fT7VPsl94rilJ8gKXP/KBbpldrfmOAdVDgUC+MHw3sSXSt+VnorB4DU4mUQLcMriQmbXdQc8d1HUZYZEkcKaSgbygHLtByOJF44XUsBotsTfZ4i/zVjnYcjgUQmwmAWD\"],\"issuer\":\"https://login.microsoftonline.com/{tenantid}/v2.0\"},{\"kty\":\"RSA\",\"use\":\"sig\",\"kid\":\"-KI3Q9nNR7bRofxmeZoXqbHZGew\",\"x5t\":\"-KI3Q9nNR7bRofxmeZoXqbHZGew\",\"n\":\"tJL6Wr2JUsxLyNezPQh1J6zn6wSoDAhgRYSDkaMuEHy75VikiB8wg25WuR96gdMpookdlRvh7SnRvtjQN9b5m4zJCMpSRcJ5DuXl4mcd7Cg3Zp1C5-JmMq8J7m7OS9HpUQbA1yhtCHqP7XA4UnQI28J-TnGiAa3viPLlq0663Cq6hQw7jYo5yNjdJcV5-FS-xNV7UHR4zAMRruMUHxte1IZJzbJmxjKoEjJwDTtcd6DkI3yrkmYt8GdQmu0YBHTJSZiz-M10CY3LbvLzf-tbBNKQ_gfnGGKF7MvRCmPA_YF_APynrIG7p4vPDRXhpG3_CIt317NyvGoIwiv0At83kQ\",\"e\":\"AQAB\",\"x5c\":[\"MIIDBTCCAe2gAwIBAgIQGQ6YG6NleJxJGDRAwAd/ZTANBgkqhkiG9w0BAQsFADAtMSswKQYDVQQDEyJhY2NvdW50cy5hY2Nlc3Njb250cm9sLndpbmRvd3MubmV0MB4XDTIyMTAwMjE4MDY0OVoXDTI3MTAwMjE4MDY0OVowLTErMCkGA1UEAxMiYWNjb3VudHMuYWNjZXNzY29udHJvbC53aW5kb3dzLm5ldDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALSS+lq9iVLMS8jXsz0IdSes5+sEqAwIYEWEg5GjLhB8u+VYpIgfMINuVrkfeoHTKaKJHZUb4e0p0b7Y0DfW+ZuMyQjKUkXCeQ7l5eJnHewoN2adQufiZjKvCe5uzkvR6VEGwNcobQh6j+1wOFJ0CNvCfk5xogGt74jy5atOutwquoUMO42KOcjY3SXFefhUvsTVe1B0eMwDEa7jFB8bXtSGSc2yZsYyqBIycA07XHeg5CN8q5JmLfBnUJrtGAR0yUmYs/jNdAmNy27y83/rWwTSkP4H5xhihezL0QpjwP2BfwD8p6yBu6eLzw0V4aRt/wiLd9ezcrxqCMIr9ALfN5ECAwEAAaMhMB8wHQYDVR0OBBYEFJcSH+6Eaqucndn9DDu7Pym7OA8rMA0GCSqGSIb3DQEBCwUAA4IBAQADKkY0PIyslgWGmRDKpp/5PqzzM9+TNDhXzk6pw8aESWoLPJo90RgTJVf8uIj3YSic89m4ftZdmGFXwHcFC91aFe3PiDgCiteDkeH8KrrpZSve1pcM4SNjxwwmIKlJdrbcaJfWRsSoGFjzbFgOecISiVaJ9ZWpb89/+BeAz1Zpmu8DSyY22dG/K6ZDx5qNFg8pehdOUYY24oMamd4J2u2lUgkCKGBZMQgBZFwk+q7H86B/byGuTDEizLjGPTY/sMms1FAX55xBydxrADAer/pKrOF1v7Dq9C1Z9QVcm5D9G4DcenyWUdMyK43NXbVQLPxLOng51KO9icp2j4U7pwHP\"],\"issuer\":\"https://login.microsoftonline.com/{tenantid}/v2.0\"},{\"kty\":\"RSA\",\"use\":\"sig\",\"kid\":\"DqUu8gf-nAgcyjP3-SuplNAXAnc\",\"x5t\":\"DqUu8gf-nAgcyjP3-SuplNAXAnc\",\"n\":\"1n7-nWSLeuWQzBRlYSbS8RjvWvkQeD7QL9fOWaGXbW73VNGH0YipZisPClFv6GzwfWECTWQp19WFe_lASka5-KEWkQVzCbEMaaafOIs7hC61P5cGgw7dhuW4s7f6ZYGZEzQ4F5rHE-YNRbvD51qirPNzKHk3nji1wrh0YtbPPIf--NbI98bCwLLh9avedOmqESzWOGECEMXv8LSM-B9SKg_4QuBtyBwwIakTuqo84swTBM5w8PdhpWZZDtPgH87Wz-_WjWvk99AjXl7l8pWPQJiKNujt_ck3NDFpzaLEppodhUsID0ptRA008eCU6l8T-ux19wZmb_yBnHcV3pFWhQ\",\"e\":\"AQAB\",\"x5c\":[\"MIIC8TCCAdmgAwIBAgIQYVk/tJ1e4phISvVrAALNKTANBgkqhkiG9w0BAQsFADAjMSEwHwYDVQQDExhsb2dpbi5taWNyb3NvZnRvbmxpbmUudXMwHhcNMjAxMjIxMDAwMDAwWhcNMjUxMjIxMDAwMDAwWjAjMSEwHwYDVQQDExhsb2dpbi5taWNyb3NvZnRvbmxpbmUudXMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDWfv6dZIt65ZDMFGVhJtLxGO9a+RB4PtAv185ZoZdtbvdU0YfRiKlmKw8KUW/obPB9YQJNZCnX1YV7+UBKRrn4oRaRBXMJsQxppp84izuELrU/lwaDDt2G5bizt/plgZkTNDgXmscT5g1Fu8PnWqKs83MoeTeeOLXCuHRi1s88h/741sj3xsLAsuH1q9506aoRLNY4YQIQxe/wtIz4H1IqD/hC4G3IHDAhqRO6qjzizBMEznDw92GlZlkO0+AfztbP79aNa+T30CNeXuXylY9AmIo26O39yTc0MWnNosSmmh2FSwgPSm1EDTTx4JTqXxP67HX3BmZv/IGcdxXekVaFAgMBAAGjITAfMB0GA1UdDgQWBBQ2r//lgTPcKughDkzmCtRlw+P9SzANBgkqhkiG9w0BAQsFAAOCAQEAsFdRyczNWh/qpYvcIZbDvWYzlrmFZc6blcUzns9zf7sUWtQZrZPu5DbetV2Gr2r3qtMDKXCUaR+pqoy3I2zxTX3x8bTNhZD9YAgAFlTLNSydTaK5RHyB/5kr6B7ZJeNIk3PRVhRGt6ybCJSjV/VYVkLR5fdLP+5GhvBESobAR/d0ntriTzp7/tLMb/oXx7w5Hu1m3I8rpMocoXfH2SH1GLmMXj6Mx1dtwCDYM6bsb3fhWRz9O9OMR6QNiTnq8q9wn1QzBAnRcswYzT1LKKBPNFSasCvLYOCPOZCL+W8N8jqa9ZRYNYKWXzmiSptgBEM24t3m5FUWzWqoLu9pIcnkPQ==\"],\"issuer\":\"https://login.microsoftonline.com/{tenantid}/v2.0\"},{\"kty\":\"RSA\",\"use\":\"sig\",\"kid\":\"OzZ5Dbmcso9Qzt2ModGmihg30Bo\",\"x5t\":\"OzZ5Dbmcso9Qzt2ModGmihg30Bo\",\"n\":\"01re9a2BUTtNtdFzLNI-QEHW8XhDiDMDbGMkxHRIYXH41zBccsXwH9vMi0HuxXHpXOzwtUYKwl93ZR37tp6lpvwlU1HePNmZpJ9D-XAvU73x03YKoZEdaFB39VsVyLih3fuPv6DPE2qT-TNE3X5YdIWOGFrcMkcXLsjO-BCq4qcSdBH2lBgEQUuD6nqreLZsg-gPzSDhjVScIUZGiD8M2sKxADiIHo5KlaZIyu32t8JkavP9jM7ItSAjzig1W2yvVQzUQZA-xZqJo2jxB3g_fygdPUHK6UN-_cqkrfxn2-VWH1wMhlm90SpxTMD4HoYOViz1ggH8GCX2aBiX5OzQ6Q\",\"e\":\"AQAB\",\"x5c\":[\"MIIC8TCCAdmgAwIBAgIQQrXPXIlUE4JMTMkVj+02YTANBgkqhkiG9w0BAQsFADAjMSEwHwYDVQQDExhsb2dpbi5taWNyb3NvZnRvbmxpbmUudXMwHhcNMjEwMzEwMDAwMDAwWhcNMjYwMzEwMDAwMDAwWjAjMSEwHwYDVQQDExhsb2dpbi5taWNyb3NvZnRvbmxpbmUudXMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDTWt71rYFRO0210XMs0j5AQdbxeEOIMwNsYyTEdEhhcfjXMFxyxfAf28yLQe7Fcelc7PC1RgrCX3dlHfu2nqWm/CVTUd482Zmkn0P5cC9TvfHTdgqhkR1oUHf1WxXIuKHd+4+/oM8TapP5M0Tdflh0hY4YWtwyRxcuyM74EKripxJ0EfaUGARBS4Pqeqt4tmyD6A/NIOGNVJwhRkaIPwzawrEAOIgejkqVpkjK7fa3wmRq8/2Mzsi1ICPOKDVbbK9VDNRBkD7FmomjaPEHeD9/KB09QcrpQ379yqSt/Gfb5VYfXAyGWb3RKnFMwPgehg5WLPWCAfwYJfZoGJfk7NDpAgMBAAGjITAfMB0GA1UdDgQWBBTECjBRANDPLGrn1p7qtwswtBU7JzANBgkqhkiG9w0BAQsFAAOCAQEAq1Ib4ERvXG5kiVmhfLOpun2ElVOLY+XkvVlyVjq35rZmSIGxgfFc08QOQFVmrWQYrlss0LbboH0cIKiD6+rVoZTMWxGEicOcGNFzrkcG0ulu0cghKYID3GKDTftYKEPkvu2vQmueqa4t2tT3PlYF7Fi2dboR5Y96Ugs8zqNwdBMRm677N/tJBk53CsOf9NnBaxZ1EGArmEHHIb80vODStv35ueLrfMRtCF/HcgkGxy2U8kaCzYmmzHh4zYDkeCwM3Cq2bGkG+Efe9hFYfDHw13DzTR+h9pPqFFiAxnZ3ofT96NrZHdYjwbfmM8cw3ldg0xQzGcwZjtyYmwJ6sDdRvQ==\"],\"issuer\":\"https://login.microsoftonline.com/{tenantid}/v2.0\"},{\"kty\":\"RSA\",\"use\":\"sig\",\"kid\":\"1LTMzakihiRla_8z2BEJVXeWMqo\",\"x5t\":\"1LTMzakihiRla_8z2BEJVXeWMqo\",\"n\":\"3sKcJSD4cHwTY5jYm5lNEzqk3wON1CaARO5EoWIQt5u-X-ZnW61CiRZpWpfhKwRYU153td5R8p-AJDWT-NcEJ0MHU3KiuIEPmbgJpS7qkyURuHRucDM2lO4L4XfIlvizQrlyJnJcd09uLErZEO9PcvKiDHoois2B4fGj7CsAe5UZgExJvACDlsQSku2JUyDmZUZP2_u_gCuqNJM5o0hW7FKRI3MFoYCsqSEmHnnumuJ2jF0RHDRWQpodhlAR6uKLoiWHqHO3aG7scxYMj5cMzkpe1Kq_Dm5yyHkMCSJ_JaRhwymFfV_SWkqd3n-WVZT0ADLEq0RNi9tqZ43noUnO_w\",\"e\":\"AQAB\",\"x5c\":[\"MIIDYDCCAkigAwIBAgIJAIB4jVVJ3BeuMA0GCSqGSIb3DQEBCwUAMCkxJzAlBgNVBAMTHkxpdmUgSUQgU1RTIFNpZ25pbmcgUHVibGljIEtleTAeFw0xNjA0MDUxNDQzMzVaFw0yMTA0MDQxNDQzMzVaMCkxJzAlBgNVBAMTHkxpdmUgSUQgU1RTIFNpZ25pbmcgUHVibGljIEtleTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAN7CnCUg+HB8E2OY2JuZTRM6pN8DjdQmgETuRKFiELebvl/mZ1utQokWaVqX4SsEWFNed7XeUfKfgCQ1k/jXBCdDB1NyoriBD5m4CaUu6pMlEbh0bnAzNpTuC+F3yJb4s0K5ciZyXHdPbixK2RDvT3Lyogx6KIrNgeHxo+wrAHuVGYBMSbwAg5bEEpLtiVMg5mVGT9v7v4ArqjSTOaNIVuxSkSNzBaGArKkhJh557pridoxdERw0VkKaHYZQEerii6Ilh6hzt2hu7HMWDI+XDM5KXtSqvw5ucsh5DAkifyWkYcMphX1f0lpKnd5/llWU9AAyxKtETYvbameN56FJzv8CAwEAAaOBijCBhzAdBgNVHQ4EFgQU9IdLLpbC2S8Wn1MCXsdtFac9SRYwWQYDVR0jBFIwUIAU9IdLLpbC2S8Wn1MCXsdtFac9SRahLaQrMCkxJzAlBgNVBAMTHkxpdmUgSUQgU1RTIFNpZ25pbmcgUHVibGljIEtleYIJAIB4jVVJ3BeuMAsGA1UdDwQEAwIBxjANBgkqhkiG9w0BAQsFAAOCAQEAXk0sQAib0PGqvwELTlflQEKS++vqpWYPW/2gCVCn5shbyP1J7z1nT8kE/ZDVdl3LvGgTMfdDHaRF5ie5NjkTHmVOKbbHaWpTwUFbYAFBJGnx+s/9XSdmNmW9GlUjdpd6lCZxsI6888r0ptBgKINRRrkwMlq3jD1U0kv4JlsIhafUIOqGi4+hIDXBlY0F/HJPfUU75N885/r4CCxKhmfh3PBM35XOch/NGC67fLjqLN+TIWLoxnvil9m3jRjqOA9u50JUeDGZABIYIMcAdLpI2lcfru4wXcYXuQul22nAR7yOyGKNOKULoOTE4t4AeGRqCogXSxZgaTgKSBhvhE+MGg==\"],\"issuer\":\"https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0\"},{\"kty\":\"RSA\",\"use\":\"sig\",\"kid\":\"bW8ZcMjBCnJZS-ibX5UQDNStvx4\",\"x5t\":\"bW8ZcMjBCnJZS-ibX5UQDNStvx4\",\"n\":\"2a70SwgqIh8U-Shj_VJJGBheEVk2F4ygmMCRtKUAb1jMP6R1j5Mc5xaqhgzlWjckJI1lx4rha1oNLrdg8tJBxdm8V8xZohCOanJ52uAwoc6FFTY3VRLaUZSJ3zCXfuJwy4KvFHJUAuLhLj0hVeq-y10CmRJ1_MPTuNRJLdblSWcXyWYIikIRggQWS04M-QjR7571mX-Lu_eDs8xJVrnNFMVGRmFqf3EFD4QLNjW9JJj0m_prnTv41V_E8AA7MQZ12ip3u5aeOAQqGjVyzdHxvV9laxta6XWaM8QSTIu_Zav1-aDYExp99nCP4Hw0_Oom5vK5N88DB8VM0mouQi8a8Q\",\"e\":\"AQAB\",\"x5c\":[\"MIIDYDCCAkigAwIBAgIJAN2X7t+ckntxMA0GCSqGSIb3DQEBCwUAMCkxJzAlBgNVBAMTHkxpdmUgSUQgU1RTIFNpZ25pbmcgUHVibGljIEtleTAeFw0yMTAzMjkyMzM4MzNaFw0yNjAzMjgyMzM4MzNaMCkxJzAlBgNVBAMTHkxpdmUgSUQgU1RTIFNpZ25pbmcgUHVibGljIEtleTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBANmu9EsIKiIfFPkoY/1SSRgYXhFZNheMoJjAkbSlAG9YzD+kdY+THOcWqoYM5Vo3JCSNZceK4WtaDS63YPLSQcXZvFfMWaIQjmpyedrgMKHOhRU2N1US2lGUid8wl37icMuCrxRyVALi4S49IVXqvstdApkSdfzD07jUSS3W5UlnF8lmCIpCEYIEFktODPkI0e+e9Zl/i7v3g7PMSVa5zRTFRkZhan9xBQ+ECzY1vSSY9Jv6a507+NVfxPAAOzEGddoqd7uWnjgEKho1cs3R8b1fZWsbWul1mjPEEkyLv2Wr9fmg2BMaffZwj+B8NPzqJubyuTfPAwfFTNJqLkIvGvECAwEAAaOBijCBhzAdBgNVHQ4EFgQU57BsETnF8TctGU87R4N9YxmNWoIwWQYDVR0jBFIwUIAU57BsETnF8TctGU87R4N9YxmNWoKhLaQrMCkxJzAlBgNVBAMTHkxpdmUgSUQgU1RTIFNpZ25pbmcgUHVibGljIEtleYIJAN2X7t+ckntxMAsGA1UdDwQEAwIBxjANBgkqhkiG9w0BAQsFAAOCAQEAcsk+LGlTzSQdnh3mtCBMNCGZCiTYvFcqenwjDf1/c4U+Yi7fxYmAXm7wVLX+GVMxpLPpzMuVOXztGoPMUgWH59CFWhsMvZbIUKsd8xbEKmls1ZIgxRYdagcWTGeBET6XIoF6Ba57BhRCxFPslhIpg27/HnfHtTdGfjRpafNbBYvC/9PL/s2E9U4AklpUn2W19UiJLRFgXGPjYPLW0j1Od0qzHHJ84saclVwvuOrpp75Y+0Du5Z2OrjNF1W4dEWZMJmmOe73ejAnoiWJI25kQpkd4ooNasw3HIZEJZ6cKctmPJLdvx0tJ8bde4DivtWOeFIwcAkokH2jlHmAOipNETw==\"],\"issuer\":\"https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0\"}]}"
	var rawMessage = json.RawMessage{}
	err := json.Unmarshal([]byte(staticJsonJwksString), &rawMessage)
	if err != nil {
		return nil, err
	}
	jwks, err := keyfunc.NewJSON(rawMessage)
	if err != nil {
		return nil, err
	}
	return jwks, nil
}

func JWKSInitAzure(ctx context.Context) (*keyfunc.JWKS, error) {
	jwksURL := "https://login.microsoftonline.com/common/discovery/v2.0/keys"
	//exhaustruct:ignore
	options := keyfunc.Options{
		Ctx: ctx,
		RefreshErrorHandler: func(err error) {
			log.Printf("There was an error with the jwt.Keyfunc. Error: %s", err.Error())
		},
		RefreshInterval:   time.Hour,
		RefreshRateLimit:  time.Minute * 5,
		RefreshTimeout:    time.Second * 10,
		RefreshUnknownKID: true,
	}
	var err error
	jwks, err := keyfunc.Get(jwksURL, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWKS from resource at the given URL: %w", err)
	}
	return jwks, nil
}

func ValidateToken(jwtB64 string, jwks *keyfunc.JWKS, clientId string, tenantId string) (jwt.MapClaims, error) {
	var token *jwt.Token
	if jwks == nil {
		return nil, fmt.Errorf("jwks not initialized")
	}
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(jwtB64, claims, jwks.Keyfunc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the JWT: %w", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token provided")
	}
	if val, ok := claims["aud"]; ok {
		if val != clientId {
			return nil, fmt.Errorf("unknown client id provided: %s", val)
		}
	} else {
		return nil, fmt.Errorf("client id not found in token")
	}

	if val, ok := claims["tid"]; ok {
		if val != tenantId {
			return nil, fmt.Errorf("unknown tenant id provided: %s", val)
		}
	} else {
		return nil, fmt.Errorf("tenant id not found in token")
	}

	return claims, nil
}

func HttpAuthMiddleWare(resp http.ResponseWriter, req *http.Request, jwks *keyfunc.JWKS, clientId string, tenantId string, allowedPaths []string, allowedPrefixes []string) error {
	token := req.Header.Get("authorization")
	if AllowBypassingAzureAuth(allowedPaths, req.URL.Path, req.Method, allowedPrefixes) {
		return nil
	}
	claims, err := ValidateToken(token, jwks, clientId, tenantId)
	if _, ok := claims["aud"]; ok && claims["aud"] == clientId {
		req.Header.Set("username", claims["name"].(string))
		req.Header.Set("email", claims["email"].(string))
	}
	if err != nil {
		resp.WriteHeader(http.StatusUnauthorized)
		_, _ = resp.Write([]byte("Invalid authorization header provided"))
	}
	return err
}

func AllowBypassingAzureAuth(allowedPaths []string, requestUrlPath string, requestMethod string, allowedPrefixes []string) bool {
	for _, allowedPath := range allowedPaths {
		if requestUrlPath == allowedPath {
			return true
		}
	}
	for _, allowedPrefix := range allowedPrefixes {
		if strings.HasPrefix(requestUrlPath, allowedPrefix) {
			return true
		}
	}

	// Skip azure authentication with ID for `/` (POST: createEnv), `/release`, `/releasetrain` and `/locks`  endpoints. The requests will be validated with pgp signature
	// usage in requests from outside the cluster (e.g. by GitHub Actions and the publish.sh script).
	group, tail := xpath.Shift(requestUrlPath)

	if group == "environments" || group == "environment-groups" {
		envName, tail := xpath.Shift(tail)
		if envName != "" { // We shouldn't receive an empty env, added just as a second layer of validation
			function, tail := xpath.Shift(tail)
			switch function {
			case "locks":
				return true
			case "releasetrain":
				return true
			case "rollout-status":
				return true
			case "": // create environment
				if tail == "/" && requestMethod == http.MethodPost {
					return true
				}
			}
		}
	}
	return false
}
