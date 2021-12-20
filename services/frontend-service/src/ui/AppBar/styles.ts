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
                width: '20%',
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
}));
