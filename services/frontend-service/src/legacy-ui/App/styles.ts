import { createTheme, makeStyles, ThemeOptions } from '@material-ui/core/styles';

export const theme = createTheme({
    palette: {
        mode: 'dark',
        primary: {
            main: '#b9ff00',
            contrastText: '#0c423f',
        },
        secondary: {
            main: '#ff00ff',
            contrastText: '#e9e9e9',
        },
        error: {
            main: '#ff9800',
        },
        success: {
            main: '#00c150',
        },
    },
    typography: {
        subtitle1: {
            color: '#c9c9c9',
        },
    },
} as ThemeOptions);

export const useStyles = makeStyles((theme) => ({
    '@global': {
        body: {
            backgroundColor: theme.palette.background.default,
            height: '100vh',
            width: '100vw',
        },
    },
    loading: {
        height: '100vh',
        width: '100vw',
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
    },
    main: {
        backgroundColor: theme.palette.background.default,
        color: theme.palette.text.primary,
        overflow: 'auto',
        '& > *': {
            height: '100vh',
            paddingTop: '48px',
        },
    },
}));
