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
import React from 'react';
import { GlobalLoadingState } from './store';
import { Spinner } from '../components/Spinner/Spinner';

export const LoadingStateSpinner: React.FC<{ loadingState: GlobalLoadingState }> = (props) => {
    const { loadingState } = props;
    if (!loadingState.configReady) {
        return <Spinner message={'Loading Configuration'} />;
    }
    if (loadingState.azureAuthEnabled && !loadingState.isAuthenticated) {
        return <Spinner message={'Authenticating with Azure'} />;
    }
    if (!loadingState.overviewLoaded) {
        return <Spinner message={'Loading Overview'} />;
    }
    return <Spinner message={'Loading'} />;
};
