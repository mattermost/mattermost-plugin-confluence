import {connect} from 'react-redux';
import {bindActionCreators} from 'redux';
import {getCurrentChannelId} from 'mattermost-redux/selectors/entities/common';

import {closeSubscriptionModal, saveChannelSubscription, editChannelSubscription, getPluginConfig} from '../../actions';
import Selectors from '../../selectors';

import SubscriptionModal from './subscription_modal';

const mapStateToProps = (state) => {
    return {
        subscription: Selectors.isSubscriptionEditModalVisible(state),
        visibility: Selectors.isSubscriptionModalVisible(state),
        currentChannelID: getCurrentChannelId(state),
    };
};

const mapDispatchToProps = (dispatch) => bindActionCreators({
    close: closeSubscriptionModal,
    saveChannelSubscription,
    editChannelSubscription,
    getPluginConfig,
}, dispatch);

export default connect(mapStateToProps, mapDispatchToProps)(SubscriptionModal);
