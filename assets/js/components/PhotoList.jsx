var React = require('react');
var Router = require('react-router');

var PaginationItem = React.createClass({

    render: function() {

        var page = this.props.page;
        var handlePaginationLink = this.props.handlePaginationLink;
        var isCurrent = (page.number === page.currentPage)
        var className = (isCurrent)? 'active' : '';

        var onClick = function(){
            handlePaginationLink(page.number);
        }

        return (
        <li className={className}><a onClick={onClick}>{page.number}</a></li>
        );
    }

});

var Pagination = React.createClass({

    render: function() {
        var photos = this.props.photos;
        var handlePaginationLink = this.props.handlePaginationLink;

        var pages = [];

        for (var i=1; i < photos.numPages + 1; i++) {
            pages.push({
                number: i,
                content: i,
                currentPage: photos.currentPage
            })
        }

        if (pages.length < 2) {
            return <ul/>;
        }

        return (

            <ul className="pagination">
                {pages.map(function(page){
                    return (
                        <PaginationItem key={page.number} page={page} handlePaginationLink={handlePaginationLink} />
                    );
                })}
            </ul>
        );

    }
});

var PhotoListItem = React.createClass({

    mixins: [Router.Navigation],

    handleSelectPhoto: function() {
        this.transitionTo("photoDetail", { id: this.props.photo.id });
    },

    render: function(){
        var photo = this.props.photo;
        return (
    <div className="col-xs-6 col-md-3" onClick={this.handleSelectPhoto}>
        <div className="thumbnail">
            <img alt={photo.title} className="img-responsive" src={'uploads/thumbnails/' + photo.photo} />
            <div className="caption">
                <h3>{photo.title}</h3>
            </div>
        </div>
    </div>
        );
    }
});

var PhotoList = React.createClass({

    render: function (){

        return (
            <div>
            <Pagination photos={this.props.photos} handlePaginationLink={this.props.handlePaginationLink} />
            <div className="row">
                {this.props.photos.photos.map(function(photo) {
                    return <PhotoListItem key={photo.id} photo={photo}  />;
                })};
            </div>
            <Pagination photos={this.props.photos} handlePaginationLink={this.props.handlePaginationLink} />
            </div>
        );
    }

})

module.exports = PhotoList;
