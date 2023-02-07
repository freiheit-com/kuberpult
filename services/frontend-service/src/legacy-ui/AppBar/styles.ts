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

Copyright 2023 freiheit.com*/
import { makeStyles } from '@material-ui/core/styles';

export const useStyles = makeStyles((theme) => ({
    environments: {
        background: theme.palette.background.default,
        padding: theme.spacing(1),
        '& .environment': {
            display: 'flex',
            flexDirection: 'row',
            alignItems: 'center',
            padding: '0px 12px',
            minHeight: '52px',
            '& .name': {
                width: '15%',
            },
            '& .big-name': {
                width: '50%',
            },
            '& .locks': {
                '& .overlay': {
                    width: '400px',
                    '& .MuiTextField-root': {
                        width: '100%',
                    },
                    '& .MuiButtonBase-root': {
                        minWidth: '91px',
                        padding: '4px 6px',
                        margin: '2px 6px 2px 0px',
                    },
                },
                '& .MuiSvgIcon-root': {
                    color: theme.palette.primary.main,
                },
            },
        },
    },
    hide: {
        opacity: 1,
    },
}));
