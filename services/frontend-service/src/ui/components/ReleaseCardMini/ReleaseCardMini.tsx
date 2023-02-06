import classNames from 'classnames';
import React from 'react';
import { updateReleaseDialog, useRelease } from '../../utils/store';
import { EnvironmentGroupChipList } from '../chip/EnvironmentGroupChip';

export type ReleaseCardMiniProps = {
    className?: string;
    version: number;
    app: string;
};

const getDays = (date: Date) => {
    const current = new Date(Date.now());
    const diff = current.getTime() - date.getTime();

    return Math.round(diff / (1000 * 3600 * 24));
};

export const ReleaseCardMini: React.FC<ReleaseCardMiniProps> = (props) => {
    const { className, app, version } = props;
    const { createdAt, sourceMessage, sourceAuthor } = useRelease(app, version);
    const clickHanlder = React.useCallback(() => {
        updateReleaseDialog(app, version);
    }, [app, version]);
    let msg = sourceAuthor;
    if (createdAt !== undefined) {
        const days = getDays(createdAt);
        msg += ' commited ';

        if (days === 0) {
            msg += 'at ';
        } else {
            msg += days + ' days ago at ';
        }
        msg += `${createdAt.getHours()}:${createdAt.getMinutes()}:${createdAt.getSeconds()}`;
    }

    return (
        <div className={classNames('release-card-mini', className)} onClick={clickHanlder}>
            <div className={classNames('release__details-mini', className)}>
                <div className="release__details-header">{sourceMessage}</div>
                <div className="release__details-msg">{msg}</div>
            </div>
            <div className="release__environments-mini">
                <EnvironmentGroupChipList app={props.app} version={props.version} />
            </div>
        </div>
    );
};
